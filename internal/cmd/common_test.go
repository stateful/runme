package cmd

import (
	"testing"

	runnerv1 "github.com/stateful/runme/v3/internal/gen/proto/go/runme/runner/v1"
	"github.com/stateful/runme/v3/internal/tui/prompt"
	"github.com/stretchr/testify/assert"
)

func TestResolveInputParams(t *testing.T) {
	variable := &runnerv1.ResolveProgramResponse_VarResult{
		Name:          "MY_VARIABLE",
		ResolvedValue: "resolved_value",
		OriginalValue: "original_value",
		Status:        runnerv1.ResolveProgramResponse_STATUS_UNRESOLVED_WITH_PLACEHOLDER,
	}

	expected := prompt.InputParams{
		Label: "Set Environment Variable \"MY_VARIABLE\":",
		Value: "original_value",
	}

	result := resolveInputParams(variable)
	assert.Equal(t, expected, result)
}
