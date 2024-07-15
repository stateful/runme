package command

import (
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"io"

	"github.com/pkg/errors"
)

type EnvEncryptor struct {
	source io.Reader
	stream cipher.Stream
}

func NewEnvEncryptor(key []byte, nonce []byte, source io.Reader) (*EnvEncryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	stream := cipher.NewCTR(block, nonce)
	return &EnvEncryptor{
		source: source,
		stream: stream,
	}, nil
}

func (e *EnvEncryptor) Read(p []byte) (n int, err error) {
	n, readErr := e.source.Read(p)
	if n > 0 {
		e.stream.XORKeyStream(p[:n], p[:n])
		return n, readErr
	}
	return 0, io.EOF
}

type EnvDecryptor struct {
	source io.Reader
	stream cipher.Stream
}

func NewEnvDecryptor(key []byte, nonce []byte, source io.Reader) (*EnvDecryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	stream := cipher.NewCTR(block, nonce)
	return &EnvDecryptor{
		source: source,
		stream: stream,
	}, nil
}

func (d *EnvDecryptor) Read(p []byte) (n int, err error) {
	n, readErr := d.source.Read(p)
	if n > 0 {
		d.stream.XORKeyStream(p[:n], p[:n])
		return n, readErr
	}
	return 0, io.EOF
}

func createEnvEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	_, err := cryptorand.Read(key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func createEnvEncryptionNonce() ([]byte, error) {
	nonce := make([]byte, aes.BlockSize)
	_, err := cryptorand.Read(nonce)
	if err != nil {
		return nil, err
	}
	return nonce, nil
}
