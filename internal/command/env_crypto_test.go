package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvEncryptorDecryptor(t *testing.T) {
	t.Parallel()

	source := "ENV_1=1\\0ENV_2=2"

	key, err := createEnvEncryptionKey()
	require.NoError(t, err)
	nonce, err := createEnvEncryptionNonce()
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
