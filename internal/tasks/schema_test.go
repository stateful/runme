package tasks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchema(t *testing.T) {
	expected := TaskConfiguration{
		Version: "2.0.0",
		BaseTaskConfiguration: BaseTaskConfiguration{
			Tasks: []TaskDescription{
				{
					Label:   "task: build extension",
					Command: "yarn",
					Args:    []string{"compile"},
					Options: &CommandOptions{
						Cwd: "./extension",
					},
					Group: "build",
				},
			},
		},
	}
	expectedBytes, err := json.Marshal(expected)
	require.NoError(t, err)

	var got TaskConfiguration
	err = json.Unmarshal(expectedBytes, &got)
	require.NoError(t, err)
	require.EqualValues(t, expected, got)
}
