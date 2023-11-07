//go:build !windows

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/assert"
)

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
		Dir: "testdata/script",
	})
}

func TestRunmeCategories(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/categories",
	})
}

func TestRunmeRunAll(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/runall",
	})
}

func TestSkipPromptsWithinAPty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = os.Setenv("RUNME_VERBOSE", "false")
	defer os.Unsetenv("RUNME_VERBOSE")

	cmd := exec.Command("go", "run", ".", "run", "skip-prompts-sample", "--chdir", "./examples/frontmatter/skipPrompts")
	ptmx, err := pty.StartWithAttrs(cmd, &pty.Winsize{Rows: 25, Cols: 80}, &syscall.SysProcAttr{})
	if err != nil {
		t.Fatalf("Could not start command with pty: %s", err)
	}
	defer ptmx.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(ptmx)

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("Command timed out")
		return
	}

	assert.Nil(t, err, "Command execution failed")

	expected := "The content of ENV is <insert-env-here>"
	current := buf.String()
	current = removeAnsiCodes(current)
	current = strings.TrimSpace(current)

	assert.Equal(t, expected, current, "Output does not match")
}

func removeAnsiCodes(str string) string {
	re := regexp.MustCompile(`\x1b\[.*?[a-zA-Z]|\x1b\].*?\x1b\\`)
	return re.ReplaceAllString(str, "")
}
