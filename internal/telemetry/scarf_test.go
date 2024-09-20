package telemetry

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestReportUnlessNoTracking(t *testing.T) {
	// don't send telemetry in tests
	reporter = func() error {
		return nil
	}

	t.Run("Track", func(t *testing.T) {
		logger := zap.NewNop()
		require.True(t, ReportUnlessNoTracking(logger))
	})

	t.Run("DO_NOT_TRACK", func(t *testing.T) {
		logger := zap.NewNop()
		t.Setenv("DO_NOT_TRACK", "true")
		require.False(t, ReportUnlessNoTracking(logger))
	})

	t.Run("SCARF_NO_ANALYTICS", func(t *testing.T) {
		logger := zap.NewNop()
		t.Setenv("SCARF_NO_ANALYTICS", "true")
		defer os.Unsetenv("SCARF_NO_ANALYTICS")
		require.False(t, ReportUnlessNoTracking(logger))
	})
}

func TestUrlBuilder(t *testing.T) {
	t.Parallel()

	t.Run("Full", func(t *testing.T) {
		lookupEnv := createLookup(map[string]string{
			"TELEMETRY_EXTNAME":    "stateful.runme",
			"TELEMETRY_EXTVERSION": "3.7.7-dev.10",
			"TELEMETRY_REMOTENAME": "none",
			"TELEMETRY_APPNAME":    "Visual Studio Code",
			"TELEMETRY_PRODUCT":    "desktop",
			"TELEMETRY_PLATFORM":   "darwin_arm64",
			"TELEMETRY_UIKIND":     "desktop",
		})
		dst, err := buildURL(lookupEnv, "Kernel")
		require.NoError(t, err)
		require.Equal(t, "https://home.runme.dev/Kernel?appname=Visual+Studio+Code&extname=stateful.runme&extversion=3.7.7-dev.10&platform=darwin_arm64&product=desktop&remotename=none&uikind=desktop", dst.String())
	})

	t.Run("Partial", func(t *testing.T) {
		lookupEnv := createLookup(map[string]string{
			"TELEMETRY_EXTNAME":  "stateful.runme",
			"TELEMETRY_PLATFORM": "linux_x64",
		})
		dst, err := buildURL(lookupEnv, "Kernel")
		require.NoError(t, err)
		require.Equal(t, "https://home.runme.dev/Kernel?extname=stateful.runme&platform=linux_x64", dst.String())
	})

	t.Run("Empty", func(t *testing.T) {
		lookupEnv := createLookup(map[string]string{})
		_, err := buildURL(lookupEnv, "Kernel")
		require.Error(t, err, "no telemetry properties provided")
	})
}

func createLookup(fixture map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := fixture[key]
		return value, ok
	}
}
