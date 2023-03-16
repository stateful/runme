package shell

import (
	"testing"

	"github.com/go-playground/assert/v2"
)

func Test_StripComments(t *testing.T) {
	cmd := []string{
		"# Commented line",
		"# Commented line # with subcomment",
		"echo Hello World # with comment",
		"echo Hello World",
	}

	assert.Equal(t, []string{
		"echo Hello World",
		"echo Hello World",
	}, StripComments(cmd))
}

func Test_TryGetNonCommentLine(t *testing.T) {
	assert.Equal(t,
		"echo Hello World",
		TryGetNonCommentLine([]string{
			"# Commented line",
			"echo Hello World",
		}),
	)

	assert.Equal(t,
		"echo Hello World",
		TryGetNonCommentLine([]string{
			"# Commented line",
			"echo Hello World #with comment",
		}),
	)

	assert.Equal(t,
		"# Commented line",
		TryGetNonCommentLine([]string{
			"# Commented line",
			"# Only Comments",
		}),
	)
}
