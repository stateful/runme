package tls

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func Test_GenerateTLS(t *testing.T) {
	tlsDir := filepath.Join(os.TempDir(), uuid.New().String())
	os.RemoveAll(tlsDir)
	os.MkdirAll(tlsDir, 0o700)
	defer os.RemoveAll(tlsDir)

	logger := zaptest.NewLogger(t)
	tlsConfig, err := GenerateTLS(tlsDir, 0o700, logger)
	require.NoError(t, err)

	tlsConfig2, err := GenerateTLS(tlsDir, 0o700, logger)
	require.NoError(t, err)

	require.Equal(
		t,
		tlsConfig.Certificates[0].Certificate[0],
		tlsConfig2.Certificates[0].Certificate[0],
	)

	oldGetNow := getNow
	defer func() {
		getNow = oldGetNow
	}()

	getNow = func() time.Time {
		return time.Now().AddDate(0, 0, 24)
	}

	tlsConfig3, err := GenerateTLS(tlsDir, 0o700, logger)
	require.NoError(t, err)

	require.NotEqual(
		t,
		tlsConfig.Certificates[0].Certificate[0],
		tlsConfig3.Certificates[0].Certificate[0],
	)
}
