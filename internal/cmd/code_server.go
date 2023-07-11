package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/muesli/cancelreader"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/runner"
	"github.com/stateful/runme/internal/tui"
	"go.uber.org/zap"
)

func codeServerCmd() *cobra.Command {
	var (
		codeServerArgs []string
		preview        bool
		install        bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Launch Runme in a headless web client",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			configDir := GetDefaultConfigHome()

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Downloading VS Code extension...")

			extensionFile, err := downloadVscodeExtension(filepath.Join(configDir, "extension_cache"), preview)
			if err != nil {
				return errors.Wrap(err, "failed to download vs code extension")
			}

			if err := runCodeServerCommand(cmd, execFile, "--install-extension", extensionFile, "--force"); err != nil {
				return errors.Wrap(err, "failed to install extension to code-server")
			}

			if install {
				return nil
			}

			if err := runCodeServerCommand(cmd, execFile); err != nil {
				return errors.Wrap(err, "failed to launch code-server")
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&codeServerArgs, "args", nil, "Extra args to pass to code-server")
	cmd.Flags().BoolVar(&preview, "preview", false, "Use preview extension instead of latest stable")
	cmd.Flags().BoolVar(&install, "install", false, "Install the extension to code-server without launching")

	return cmd
}

func runCodeServerCommand(cmd *cobra.Command, execFile string, args ...string) error {
	codeServerCmd := exec.Command(execFile, args...)

	codeServerCmd.Stdout = cmd.OutOrStdout()
	codeServerCmd.Stderr = cmd.ErrOrStderr()
	codeServerCmd.Stdin = cmd.InOrStdin()

	if err := codeServerCmd.Run(); err != nil {
		return errors.Wrap(err, "code-server command failed")
	}

	return nil
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
