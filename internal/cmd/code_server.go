package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/muesli/cancelreader"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/v3/internal/runner"
	"github.com/stateful/runme/v3/internal/tui"
	"go.uber.org/zap"
)

var vscodeVersionRegexp = regexp.MustCompile(`VS Code Server (\d+)\.(\d+)`)

func codeServerCmd() *cobra.Command {
	var (
		userCodeServerArgs   []string
		preview              bool
		install              bool
		open                 bool
		codeServerConfigFile string
		codeServerTitle      string
	)

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Launch Runme in a headless web client",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := getProject()
			if err != nil {
				return errors.Wrap(err, "failed to get project")
			}

			openDir := proj.Root()

			if len(args) > 0 {
				openDir = args[0]
			}

			fi, err := os.Stat(openDir)
			if err != nil {
				return errors.Wrapf(err, "directory or file %q does not exist", openDir)
			}

			// TODO: eventually we need to support opening files
			if !fi.IsDir() {
				openDir = filepath.Dir(openDir)
			}

			execFile, err := exec.LookPath("code-server")
			if err != nil {
				if !isTerminal(os.Stdout.Fd()) {
					return errors.New("no code-server installation found")
				}

				model := tui.NewStandaloneQuestionModel(
					"No code-server installation found. Do you want to install coder's code-server?",
					tui.MinimalKeyMap,
					tui.DefaultStyles,
				)

				finalModel, err := newProgram(cmd, model).Run()
				if err != nil {
					return errors.Wrap(err, "cli program failed")
				}

				confirm := finalModel.(tui.StandaloneQuestionModel).Confirmed()

				if !confirm {
					return errors.New("cancelled")
				}

				cwd, err := os.Getwd()
				if err != nil {
					return errors.Wrap(err, "failed to get cwd")
				}

				stdin, err := cancelreader.NewReader(cmd.InOrStdin())
				if err != nil {
					return errors.Wrap(err, "failed to allocate stdin")
				}

				cfg := &runner.ExecutableConfig{
					Name:   "",
					Dir:    cwd,
					Tty:    true,
					Stdout: cmd.OutOrStdout(),
					Stderr: cmd.ErrOrStderr(),
					Stdin:  stdin,
					Logger: zap.NewNop(),
				}

				installScript := "curl -fsSL https://code-server.dev/install.sh | sh"

				executable := runner.Shell{
					ExecutableConfig: cfg,
					Cmds: []string{
						installScript,
					},
				}

				// TODO(mxs): eventually we need to use StdinPipe()
				go func() {
					for {
						if executable.ExitCode() > -1 {
							stdin.Cancel()
							return
						}

						time.Sleep(100 * time.Millisecond)
					}
				}()

				if err := executable.Run(context.Background()); err != nil {
					return errors.Wrap(err, "installation command failed")
				}

				execFile, err = exec.LookPath("code-server")
				if err != nil {
					return errors.Wrap(err, "code-server still not found; you may need to refresh your shell shell environment")
				}
			}

			output, err := runCodeServerCommand(cmd, execFile, true, "--version")
			if err != nil {
				return errors.Wrap(err, "failed to get code-server version")
			}

			if vscodeVersionRegexp.Match(output) {
				return errors.New("currently, we only support coder's code server; please uninstall any other code-server installations to use this feature")
			}

			configDir := filepath.Join(GetUserConfigHome(), "code-server")

			if codeServerConfigFile == "" {
				codeServerConfigFile = filepath.Join(configDir, "config.yaml")
			}

			if _, err := os.Stat(codeServerConfigFile); os.IsNotExist(err) {
				if err := os.MkdirAll(configDir, 0o700); err != nil {
					return errors.Wrap(err, "failed to create config directory")
				}

				defaultConfig := bytes.Join(
					[][]byte{
						[]byte("bind-addr: 127.0.0.1:8080"),
						[]byte("auth: none"),
						[]byte("cert: false"),
					},
					[]byte("\n"),
				)

				if err := os.WriteFile(codeServerConfigFile, defaultConfig, 0o600); err != nil {
					return errors.Wrap(err, "failed to create config file")
				}
			}

			if _, err := runCodeServerCommand(cmd, execFile, false, "--install-extension", fExtensionHandle, "--force"); err != nil {
				return errors.Wrap(err, "failed to install extension to code-server")
			}

			if install {
				return nil
			}

			codeServerArgs := []string{"--disable-getting-started-override"}

			if open {
				codeServerArgs = append(codeServerArgs, "--open")
			}

			if codeServerConfigFile != "" {
				codeServerArgs = append(codeServerArgs, "--config", codeServerConfigFile)
			}

			if codeServerTitle != "" {
				codeServerArgs = append(codeServerArgs, "--app-name", codeServerTitle)
			}

			if openDir != "" {
				codeServerArgs = append(codeServerArgs, openDir)
			}

			// this command flags forces code-server to open the provided directory,
			// even if the user opened a different workspace before
			codeServerArgs = append(codeServerArgs, "-e")

			codeServerArgs = append(codeServerArgs, userCodeServerArgs...)

			if _, err := runCodeServerCommand(cmd, execFile, false, codeServerArgs...); err != nil {
				return errors.Wrap(err, "failed to launch code-server")
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&userCodeServerArgs, "args", nil, "Extra args to pass to code-server")
	cmd.Flags().BoolVar(&preview, "preview", false, "Use preview extension instead of latest stable")
	cmd.Flags().BoolVar(&install, "install", false, "Install the extension to code-server without launching")
	cmd.Flags().BoolVar(&open, "open", true, "Automatically open the code server in the browser on startup")
	cmd.Flags().StringVar(&codeServerConfigFile, "config", "", "Path to code-server config file")
	cmd.Flags().StringVar(&codeServerTitle, "title", "Runme", "Title of the code-server window/session")

	return cmd
}

func runCodeServerCommand(cmd *cobra.Command, execFile string, routeToBuffer bool, args ...string) ([]byte, error) {
	codeServerCmd := exec.Command(execFile, args...)
	buffer := bytes.NewBuffer(nil)

	if routeToBuffer {
		codeServerCmd.Stdout = buffer
	} else {
		codeServerCmd.Stdout = cmd.OutOrStdout()
	}

	codeServerCmd.Stderr = cmd.ErrOrStderr()
	codeServerCmd.Stdin = cmd.InOrStdin()

	if err := codeServerCmd.Run(); err != nil {
		return nil, errors.Wrap(err, "code-server command failed")
	}

	return buffer.Bytes(), nil
}

func getLatestExtensionVersion(experimental bool) (string, error) {
	var tagName string

	var suffix string

	if !experimental {
		suffix = "/latest"
	}

	versionListURL := fmt.Sprintf("https://api.github.com/repos/stateful/vscode-runme/releases%v", suffix)

	req, err := http.NewRequest("GET", versionListURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var dataResp interface{}
	if err := json.Unmarshal(body, &dataResp); err != nil {
		return "", errors.Wrap(err, "failed parsing JSON")
	}

	switch respObj := dataResp.(type) {
	case map[string]interface{}:
		tagName = respObj["tag_name"].(string)

	case []interface{}:
		tagName = respObj[0].(map[string]interface{})["tag_name"].(string)

	default:
		return "", errors.New("unexpected github API schema")
	}

	return tagName, nil
}

func getExtensionURL(tagName string) (string, error) {
	var arch string

	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"

	case "arm64":
		arch = "arm64"

	default:
		return "", fmt.Errorf("unsupported cpu architecture %v", arch)
	}

	var platform string

	switch runtime.GOOS {
	case "windows":
		platform = "win32"

	case "linux":
		platform = "linux"

	case "darwin":
		platform = "darwin"
	}

	tagNameMin := strings.Replace(tagName, "v", "", 1)

	binary := fmt.Sprintf("runme-%s-%s-%s.vsix", platform, arch, tagNameMin)

	downloadURL := fmt.Sprintf("https://github.com/stateful/vscode-runme/releases/download/%s/%s", tagNameMin, binary)

	return downloadURL, nil
}

func downloadVscodeExtension(dir string, experimental bool) (string, error) {
	tagName, err := getLatestExtensionVersion(experimental)
	if err != nil {
		return "", err
	}

	cacheDir := filepath.Join(dir, tagName)
	fileName := filepath.Join(cacheDir, "vscode-runme.vsix")

	if info, err := os.Stat(fileName); err == nil {
		if !info.IsDir() {
			return fileName, nil
		}
	}

	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}

	vsixURL, err := getExtensionURL(tagName)
	if err != nil {
		return "", err
	}

	// download extension...
	resp, err := http.Get(vsixURL) // #nosec G107
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	out, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return fileName, nil
}
