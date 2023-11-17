package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/document/identity"
	runnerv1 "github.com/stateful/runme/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/internal/project"
	"github.com/stateful/runme/internal/runner/client"
	"github.com/stateful/runme/internal/tui"
	"github.com/stateful/runme/internal/tui/prompt"
	"golang.org/x/exp/slices"
)

const envStackDepth = "__RUNME_STACK_DEPTH"

func getIdentityResolver() *identity.IdentityResolver {
	return identity.NewResolver(identity.DefaultLifecycleIdentity)
}

func getProject() (*project.Project, error) {
	opts := []project.ProjectOption{
		project.WithIdentityResolver(getIdentityResolver()),
	}

	var proj *project.Project

	if fFileMode {
		var err error
		proj, err = project.NewFileProject(filepath.Join(fChdir, fFileName), opts...)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		var err error

		projDir := fProject
		// TODO(adamb): wrap it in a function that will explain the logic
		if projDir == "" {
			projDir, err = os.Getwd()
			if err != nil {
				return nil, errors.WithMessage(err, "failed to get cwd")
			}

			opts = append(opts, project.WithFindRepoUpward())
		}

		opts = append(
			opts,
			project.WithRespectGitignore(),
			project.WithIgnoreFilePatterns(fProjectIgnorePatterns...),
		)

		if fLoadEnv && fEnvOrder != nil {
			opts = append(opts, project.WithEnvFilesReadOrder(fEnvOrder))
		}

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

func GetDefaultConfigHome() string {
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

const tlsFileMode = os.FileMode(int(0o700))

var defaultTLSDir = filepath.Join(GetDefaultConfigHome(), "tls")

func promptEnvVars(cmd *cobra.Command, envs []string, tasks ...project.Task) error {
	keys := make([]string, len(envs))

	for i, line := range envs {
		if line == "" {
			continue
		}

		keys[i] = strings.SplitN(line, "=", 2)[0]
	}

	for _, task := range tasks {
		block := task.CodeBlock

		if block.PromptEnv() {
			varPrompts := getCommandExportExtractMatches(block.Lines())
			for _, ev := range varPrompts {
				if slices.Contains(keys, ev.Key) {
					block.GetBlock().SetLine(ev.LineNumber, "")

					continue
				}

				newVal, err := promptForEnvVar(cmd, ev)
				block.GetBlock().SetLine(ev.LineNumber, replaceVarValue(ev, newVal))

				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func getCommandExportExtractMatches(lines []string) []CommandExportExtractMatch {
	test := regexp.MustCompile(exportExtractRegex)
	result := []CommandExportExtractMatch{}

	for i, line := range lines {
		for _, match := range test.FindAllStringSubmatch(line, -1) {
			e := match[0]

			parts := strings.SplitN(strings.TrimSpace(e)[len("export "):], "=", 2)
			if len(parts) == 0 {
				continue
			}
			key := parts[0]
			ph := strings.TrimSpace(parts[1])

			isExecValue := strings.HasPrefix(ph, "$(") && strings.HasSuffix(ph, ")")
			if isExecValue {
				continue
			}

			hasStringValue := strings.HasPrefix(ph, "\"") || strings.HasPrefix(ph, "'")
			placeHolder := ph
			if hasStringValue {
				placeHolder = ph[1 : len(ph)-1]
			}

			value := placeHolder

			result = append(result, CommandExportExtractMatch{
				Key:            key,
				Value:          value,
				Match:          e,
				HasStringValue: hasStringValue,
				LineNumber:     i,
			})
		}
	}

	return result
}

func promptForEnvVar(cmd *cobra.Command, ev CommandExportExtractMatch) (string, error) {
	label := fmt.Sprintf("Set Environment Variable %q:", ev.Key)
	ip := prompt.InputParams{Label: label, PlaceHolder: ev.Value}
	if ev.HasStringValue {
		ip.Value = ev.Value
	}
	model := tui.NewStandaloneInputModel(ip, tui.MinimalKeyMap, tui.DefaultStyles)

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

func replaceVarValue(ev CommandExportExtractMatch, newValue string) string {
	parts := strings.SplitN(ev.Match, "=", 2)
	replacedText := fmt.Sprintf("%s=%q", parts[0], newValue)
	return replacedText
}
