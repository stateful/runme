package tls

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestLoadOrGenerateConfig(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	logger := zaptest.NewLogger(t)

	tlsConfig1, err := LoadOrGenerateConfig(certFile, keyFile, logger)
	require.NoError(t, err)

	tlsConfig2, err := LoadOrGenerateConfig(certFile, keyFile, logger)
	require.NoError(t, err)

	require.Equal(
		t,
		tlsConfig1.Certificates[0].Certificate[0],
		tlsConfig2.Certificates[0].Certificate[0],
	)

	nowFn = func() time.Time {
		return time.Now().AddDate(0, 0, 24)
	}

	tlsConfig3, err := LoadOrGenerateConfig(certFile, keyFile, logger)
	require.NoError(t, err)

	require.NotEqual(
		t,
		tlsConfig1.Certificates[0].Certificate[0],
		tlsConfig3.Certificates[0].Certificate[0],
	)
}
