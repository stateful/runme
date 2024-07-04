package command

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvEncryptorDecryptor(t *testing.T) {
	t.Parallel()

	source := "ENV_1=1\\0ENV_2=2"

	key, err := CreateKey()
	require.NoError(t, err)
	nonce, err := CreateNonce()
	require.NoError(t, err)

	encryptor, err := NewEnvEncryptor(key, nonce, strings.NewReader(source))
	require.NoError(t, err)

	decryptor, err := NewEnvDecryptor(key, nonce, encryptor)
	require.NoError(t, err)

	var result bytes.Buffer
	_, err = result.ReadFrom(decryptor)
	require.NoError(t, err)
	require.Equal(t, source, result.String())
}

func TestEnvCollector(t *testing.T) {
	t.Parallel()

	backend, err := newEnvCollectorFileStorer()
	require.NoError(t, err)

	err = os.WriteFile(backend.PrePath(), []byte("ENV_1=1"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(backend.PostPath(), []byte("ENV_2=2"), 0o600)
	require.NoError(t, err)

	colector := newEnvCollector(backend)
	changedEnv, deletedEnv, err := colector.Diff()
	require.NoError(t, err)
	require.Equal(t, []string{"ENV_2=2"}, changedEnv)
	require.Equal(t, []string{"ENV_1"}, deletedEnv)
}
