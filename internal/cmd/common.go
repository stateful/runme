package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stateful/runme/v3/internal/runner/client"
	"github.com/stateful/runme/v3/internal/tui"
	"github.com/stateful/runme/v3/internal/tui/prompt"
	runnerv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/document/identity"
	"github.com/stateful/runme/v3/pkg/project"
)

const envStackDepth = "__RUNME_STACK_DEPTH"

func getIdentityResolver() *identity.IdentityResolver {
	return identity.NewResolver(identity.DefaultLifecycleIdentity)
}

func getProject() (*project.Project, error) {
	logger, err := getLogger(false, false)
	if err != nil {
		return nil, err
	}

	opts := []project.ProjectOption{
		project.WithLogger(logger),
	}

	var proj *project.Project

	if fFileMode {
		var err error

		filePath := filepath.Join(fChdir, fFileName)
		_, err = os.Stat(fFileName)
		if filepath.IsAbs(fFileName) && !os.IsNotExist(err) {
			// don't return error continue with NewFileProject
			filePath = fFileName
		}

		proj, err = project.NewFileProject(filePath, opts...)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		projDir := fProject
		// If no project directory is specified, use the current directory.
		// "." is a valid project directory, similarly to "..".
		if projDir == "" {
			projDir = "."
		}

		opts = append(
			opts,
			project.WithIgnoreFilePatterns(fProjectIgnorePatterns...),
			project.WithRespectGitignore(fRespectGitignore),
		)

		// By default, commands try to find repo upward unless project is non-empty.
		if fProject == "" {
			opts = append(
				opts,
				project.WithFindRepoUpward(),
			)
		}

		if fLoadEnv && fEnvOrder != nil {
			opts = append(opts, project.WithEnvFilesReadOrder(fEnvOrder))
		}

		var err error
		proj, err = project.NewDirProject(projDir, opts...)
		if err != nil {
			return nil, err
		}
	}

	return proj, nil
}

func getProjectFiles(cmd *cobra.Command) ([]string, error) {
	proj, err := getProject()
	if err != nil {
		return nil, err
	}

	loader, err := newProjectLoader(cmd, fAllowUnknown, fAllowUnnamed)
	if err != nil {
		return nil, err
	}

	return loader.LoadFiles(proj)
}

func getProjectTasks(cmd *cobra.Command) ([]project.Task, error) {
	proj, err := getProject()
	if err != nil {
		return nil, err
	}

	loader, err := newProjectLoader(cmd, fAllowUnknown, fAllowUnnamed)
	if err != nil {
		return nil, err
	}

	return loader.LoadTasks(proj)
}

func getAllProjectTasks(cmd *cobra.Command) ([]project.Task, error) {
	proj, err := getProject()
	if err != nil {
		return nil, err
	}

	loader, err := newProjectLoader(cmd, fAllowUnknown, fAllowUnnamed)
	if err != nil {
		return nil, err
	}

	return loader.LoadAllTasks(proj)
}

func getCodeBlocks() (document.CodeBlocks, error) {
	source, err := os.ReadFile(filepath.Join(fChdir, fFileName))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	doc := document.New(source, getIdentityResolver())

	node, err := doc.Root()
	if err != nil {
		return nil, err
	}

	return document.CollectCodeBlocks(node), nil
}

func getLogger(devMode bool, aiLogs bool) (*zap.Logger, error) {
	if !fLogEnabled && !aiLogs {
		return zap.NewNop(), nil
	}

	cores := make([]zapcore.Core, 0, 2)
	if fLogEnabled {
		consoleCore, err := createCoreForConsole(devMode)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		cores = append(cores, consoleCore)
	}

	aiLogFile := ""
	if aiLogs {
		aiCore, newLogFile, err := createAICoreLogger()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		aiLogFile = newLogFile
		cores = append(cores, aiCore)
	}

	if len(cores) == 0 {
		return zap.NewNop(), nil
	}

	// Create a multi-core logger with different encodings
	core := zapcore.NewTee(cores...)

	// Create the logger
	newLogger := zap.New(core)
	// Record the caller of the log message
	newLogger = newLogger.WithOptions(zap.AddCaller())

	newLogger.Info("Logger initialized", zap.Bool("devMode", devMode), zap.Bool("aiLogs", aiLogs), zap.String("aiLogFile", aiLogFile))
	return newLogger, nil
}

// createCorForConsole creates a zapcore.Core for console output.
func createCoreForConsole(devMode bool) (zapcore.Core, error) {
	if !fLogEnabled {
		return zapcore.NewNopCore(), nil
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	lvl := zap.NewAtomicLevelAt(zap.InfoLevel)

	if devMode {
		lvl = zap.NewAtomicLevelAt(zap.DebugLevel)
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	path := "stderr"
	if fLogFilePath != "" {
		path = fLogFilePath
	}

	oFile, _, err := zap.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create writer for console logger")
	}

	var encoder zapcore.Encoder
	if devMode {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	core := zapcore.NewCore(encoder, zapcore.AddSync(oFile), lvl)

	if !devMode {
		// For non-dev mode, add sampling.
		core = zapcore.NewSamplerWithOptions(
			core,
			time.Second,
			100,
			100,
		)
	}
	return core, nil
}

// createAICoreLogger creates a core logger that writes logs to files. These logs are always written in JSON
// format. Their purpose is to capture AI traces that we use for retraining. Since these are supposed to be machine
// readable they are always written in JSON format.
func createAICoreLogger() (zapcore.Core, string, error) {
	// Configure encoder for JSON format
	c := zap.NewProductionEncoderConfig()
	// We attach the function key to the logs because that is useful for identifying the function that generated the log.
	c.FunctionKey = "function"

	jsonEncoder := zapcore.NewJSONEncoder(c)

	configDir := getConfigDir()
	if configDir == "" {
		return nil, "", errors.New("could not determine config directory")
	}
	logDir := filepath.Join(configDir, "logs")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		// Logger won't be setup yet so we can't use it.
		if _, err := fmt.Fprintf(os.Stdout, "Creating log directory %s\n", logDir); err != nil {
			return nil, "", errors.Wrapf(err, "could not write to stdout")
		}
		err := os.MkdirAll(logDir, 0o750)
		if err != nil {
			return nil, "", errors.Wrapf(err, "could not create log directory %s", logDir)
		}
	}

	// We need to set a unique file name for the logs as a way of dealing with log rotation.
	name := fmt.Sprintf("logs.%s.json", time.Now().Format("2006-01-02T15:04:05"))
	logFile := filepath.Join(logDir, name)

	// TODO(jeremy): How could we handle invoking the log closer if there is one.
	oFile, _, err := zap.Open(logFile)
	if err != nil {
		return nil, logFile, errors.Wrapf(err, "could not open log file %s", logFile)
	}

	// Force log level to be at least info. Because info is the level at which we capture the logs we need for
	// tracing.
	core := zapcore.NewCore(jsonEncoder, zapcore.AddSync(oFile), zapcore.InfoLevel)

	return core, logFile, nil
}

func validCmdNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	blocks, err := getCodeBlocks()
	if err != nil {
		cmd.PrintErrf("failed to get parser: %s", err)
		return nil, cobra.ShellCompDirectiveError
	}

	names := blocks.Names()

	var filtered []string
	for _, name := range names {
		if strings.HasPrefix(name, toComplete) {
			filtered = append(filtered, name)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

func setDefaultFlags(cmd *cobra.Command) {
	usage := "Help for "
	if n := cmd.Name(); n != "" {
		usage += n
	} else {
		usage += "this command"
	}
	cmd.Flags().BoolP("help", "h", false, usage)

	// For the root command, set up the --version flag.
	if cmd.Use == "runme" {
		usage := "Version of "
		if n := cmd.Name(); n != "" {
			usage += n
		} else {
			usage += "this command"
		}
		cmd.Flags().BoolP("version", "v", false, usage)
	}
}

func printfInfo(msg string, args ...any) {
	var buf bytes.Buffer
	_, _ = buf.WriteString("\x1b[0;32m")
	_, _ = fmt.Fprintf(&buf, msg, args...)
	_, _ = buf.WriteString("\x1b[0m")
	_, _ = buf.WriteString("\r\n")
	_, _ = os.Stderr.Write(buf.Bytes())
}

// GetUserConfigHome returns the user's configuration directory.
// The user configuration directory should be used for configuration that is specific to the user and thus
// shouldn't be included in project/repository configuration. An example of user location is where server logs
// should be stored.
func GetUserConfigHome() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	_, fErr := os.Stat(dir)
	if os.IsNotExist(fErr) {
		mkdErr := os.MkdirAll(dir, 0o700)
		if mkdErr != nil {
			dir = os.TempDir()
		}
	}
	return filepath.Join(dir, "runme")
}

var (
	fLoadEnv  bool
	fEnvOrder []string
)

func setRunnerFlags(cmd *cobra.Command, serverAddr *string) func() ([]client.RunnerOption, error) {
	var (
		SessionID                 string
		SessionStrategy           string
		TLSDir                    string
		EnableBackgroundProcesses bool
	)

	cmd.Flags().StringVarP(serverAddr, "server", "s", os.Getenv("RUNME_SERVER_ADDR"), "Server address to connect runner to")
	cmd.Flags().StringVar(&SessionID, "session", os.Getenv("RUNME_SESSION"), "Session id to run commands in runner inside of")

	cmd.Flags().BoolVar(&fLoadEnv, "load-env", true, "Load env files from local project. Control which files to load with --env-order")
	cmd.Flags().StringArrayVar(&fEnvOrder, "env-order", []string{".env.local", ".env"}, "List of environment files to load in order.")

	cmd.Flags().BoolVar(&EnableBackgroundProcesses, "background", false, "Enable running background blocks as background processes")

	cmd.Flags().StringVar(&SessionStrategy, "session-strategy", func() string {
		if val, ok := os.LookupEnv("RUNME_SESSION_STRATEGY"); ok {
			return val
		}

		return "manual"
	}(), "Strategy for session selection. Options are manual, recent. Defaults to manual")

	cmd.Flags().StringVar(&TLSDir, "tls", func() string {
		if val, ok := os.LookupEnv("RUNME_TLS_DIR"); ok {
			return val
		}

		return defaultTLSDir
	}(), "Directory for TLS authentication")

	_ = cmd.Flags().MarkHidden("session")
	_ = cmd.Flags().MarkHidden("session-strategy")

	getRunOpts := func() ([]client.RunnerOption, error) {
		dir, _ := filepath.Abs(fChdir)

		if !fFileMode {
			dir, _ = filepath.Abs(fProject)
		}

		stackDepth := 0
		if depthStr, ok := os.LookupEnv(envStackDepth); ok {
			if depth, err := strconv.Atoi(depthStr); err == nil {
				stackDepth = depth + 1
			}
		}

		// TODO(mxs): user-configurable
		if stackDepth > 100 {
			panic("runme stack depth limit exceeded")
		}

		runOpts := []client.RunnerOption{
			client.WithDir(dir),
			client.WithSessionID(SessionID),
			client.WithCleanupSession(SessionID == ""),
			client.WithTLSDir(TLSDir),
			client.WithInsecure(fInsecure),
			client.WithEnableBackgroundProcesses(EnableBackgroundProcesses),
			client.WithEnvs([]string{fmt.Sprintf("%s=%d", envStackDepth, stackDepth)}),
			client.WithEnvStoreType(runnerv1.SessionEnvStoreType_SESSION_ENV_STORE_TYPE_UNSPECIFIED),
		}

		switch strings.ToLower(SessionStrategy) {
		case "manual":
			runOpts = append(runOpts, client.WithSessionStrategy(runnerv1.SessionStrategy_SESSION_STRATEGY_UNSPECIFIED))
		case "recent":
			runOpts = append(runOpts, client.WithSessionStrategy(runnerv1.SessionStrategy_SESSION_STRATEGY_MOST_RECENT))
		default:
			return nil, fmt.Errorf("unknown session strategy %q", SessionStrategy)
		}

		return runOpts, nil
	}

	return getRunOpts
}

type runFunc func(context.Context) error

var defaultTLSDir = filepath.Join(GetUserConfigHome(), "tls")

func promptEnvVars(cmd *cobra.Command, runner client.Runner, tasks ...project.Task) error {
	for _, task := range tasks {
		block := task.CodeBlock

		mode := resolveRequestMode(block.PromptEnvStr())

		script := string(block.Content())
		response, err := runner.ResolveProgram(cmd.Context(), *mode, script, block.Language())
		if err != nil {
			return err
		}

		var newLines []string
		block.SetLines(strings.Split(response.Script, "\n"))

		for _, variable := range response.Vars {
			capturedValue := ""
			switch variable.Status {
			case
				runnerv1.ResolveProgramResponse_STATUS_RESOLVED:
				capturedValue = variable.ResolvedValue
			case
				runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_MESSAGE,
				runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_PLACEHOLDER,
				runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_SECRET:
				params := resolveInputParams(variable)
				newVal := ""

				if isTerminal(os.Stdout.Fd()) {
					newVal, err = captureVariable(cmd, &params)
					if err != nil {
						return err
					}
				}

				capturedValue = newVal
			}

			if len(capturedValue) > 0 {
				newLine := fmt.Sprintf(`export %s="%s"`, variable.Name, capturedValue)
				newLines = append(newLines, newLine)
			}
		}

		if len(newLines) > 0 {
			block.PrependLines(newLines)
		}
	}

	return nil
}

func resolveRequestMode(cellMode string) *runnerv1.ResolveProgramRequest_Mode {
	var mode runnerv1.ResolveProgramRequest_Mode

	switch strings.ToLower(cellMode) {
	case "auto":
		mode = runnerv1.ResolveProgramRequest_MODE_UNSPECIFIED
	case "1", "true", "yes":
		mode = runnerv1.ResolveProgramRequest_MODE_PROMPT_ALL
	case "0", "false", "no":
		mode = runnerv1.ResolveProgramRequest_MODE_SKIP_ALL
	default:
		mode = runnerv1.ResolveProgramRequest_MODE_UNSPECIFIED
	}

	return &mode
}

func resolveInputParams(variable *runnerv1.ResolveProgramResponse_VarResult) prompt.InputParams {
	label := fmt.Sprintf("Set Environment Variable %q:", variable.Name)

	var placeHolder string

	if variable.ResolvedValue != "" {
		placeHolder = variable.ResolvedValue
	} else if variable.OriginalValue != "" {
		placeHolder = variable.OriginalValue
	} else {
		placeHolder = "Enter a value please"
	}

	ip := prompt.InputParams{Label: label}

	switch variable.Status {
	case
		runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_PLACEHOLDER,
		runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_SECRET:
		ip.Value = variable.OriginalValue
	case
		runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_MESSAGE:
		ip.PlaceHolder = placeHolder
	default:
		ip.Value = variable.ResolvedValue
	}

	return ip
}

func captureVariable(cmd *cobra.Command, ip *prompt.InputParams) (string, error) {
	model := tui.NewStandaloneInputModel(*ip, tui.MinimalKeyMap, tui.DefaultStyles)
	finalModel, err := newProgram(cmd, model).Run()
	if err != nil {
		return "", err
	}
	val, ok := finalModel.(tui.StandaloneInputModel).Value()
	if !ok {
		return "", errors.New("canceled")
	}

	return val, nil
}

func getRelativePath(base string, documentPath string) string {
	relPath, err := filepath.Rel(base, documentPath)
	if err != nil {
		relPath = documentPath
	}
	return relPath
}
