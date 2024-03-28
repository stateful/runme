package tls

import (
	"os"
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

func TestLoadOrGenerateConfigFromDir(t *testing.T) {
	dir := t.TempDir()
	logger := zaptest.NewLogger(t)

	t.Run("DirNotExist", func(t *testing.T) {
		_, err := LoadOrGenerateConfigFromDir(filepath.Join(dir, "not", "exist"), logger)
		require.NoError(t, err)
	})

	t.Run("PathNotDir", func(t *testing.T) {
		file := filepath.Join(dir, "file.txt")
		err := os.WriteFile(file, []byte("test"), 0o600)
		require.NoError(t, err)

		_, err = LoadOrGenerateConfigFromDir(file, logger)
		require.Error(t, err)
	})

	t.Run("DirExist", func(t *testing.T) {
		_, err := LoadOrGenerateConfigFromDir(filepath.Join(dir, "not", "exist"), logger)
		require.NoError(t, err)
	})
}
