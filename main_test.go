//go:build !windows

package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"testing"

	"github.com/creack/pty"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/require"
)

func isDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	paths := []string{"/proc/1/cgroup", "/proc/self/cgroup"}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		isDocker := false
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "docker") || strings.Contains(scanner.Text(), "kubepods") {
				isDocker = true
				break
			}
		}

		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return false
		}

		_ = file.Close()

		if isDocker {
			return true
		}
	}

	return false
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"runme": root,
	}))
}

// TestRunme tests runme end-to-end using testscript.
// Check out the package from "import" to learn more.
// More comprehensive tutorial can be found here:
// https://bitfieldconsulting.com/golang/test-scripts
func TestRunme(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:             "testdata/script",
		ContinueOnError: true,
	})
}

func TestRunmeFilePermissions(t *testing.T) {
	if isDocker() {
		return
	}

	testscript.Run(t, testscript.Params{
		Dir:             "testdata/permissions",
		ContinueOnError: true,
	})
}

func TestRunmeFlags(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:             "testdata/flags",
		ContinueOnError: true,
	})
}

func TestRunmeCategories(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:             "testdata/categories",
		ContinueOnError: true,
	})
}

func TestRunmeRunAll(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:             "testdata/runall",
		ContinueOnError: true,
	})
}

func TestRunmeBeta(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:             "testdata/beta",
		ContinueOnError: true,
	})
}

func TestSkipPromptsWithinAPty(t *testing.T) {
	err := os.Setenv("RUNME_VERBOSE", "false")
	require.NoError(t, err)
	defer os.Unsetenv("RUNME_VERBOSE")

	cmd := exec.Command("go", "run", ".", "run", "skip-prompts-sample", "--chdir", "./examples/frontmatter/skipPrompts")
	ptmx, err := pty.StartWithAttrs(cmd, &pty.Winsize{Rows: 25, Cols: 80}, &syscall.SysProcAttr{})
	require.NoError(t, err)
	defer ptmx.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(ptmx) // ignoring errors explicitly

	expected := "The content of ENV is <insert-env-here>"
	current := buf.String()
	current = removeAnsiCodes(current)
	current = strings.TrimSpace(current)
	require.Equal(t, expected, current, "output does not match")
}

func removeAnsiCodes(str string) string {
	re := regexp.MustCompile(`\x1b\[.*?[a-zA-Z]|\x1b\].*?\x1b\\`)
	return re.ReplaceAllString(str, "")
}
