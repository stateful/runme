package command

import (
	"io"
	runtimestd "runtime"

	"github.com/pkg/errors"
)

type EnvCollectorFactory struct {
	encryptionEnabled bool
	useFifo           bool
}

func NewEnvCollectorFactory() *EnvCollectorFactory {
	return &EnvCollectorFactory{
		encryptionEnabled: envCollectorEnableEncryption,
		useFifo:           envCollectorUseFifo,
	}
}

func (f *EnvCollectorFactory) WithEncryption(value bool) *EnvCollectorFactory {
	f.encryptionEnabled = value
	return f
}

func (f *EnvCollectorFactory) UseFifo(value bool) *EnvCollectorFactory {
	f.useFifo = value
	return f
}

func (f *EnvCollectorFactory) Build() (envCollector, error) {
	scanner := scanEnv

	var (
		encKey   []byte
		encNonce []byte
	)

	if f.encryptionEnabled {
		var err error

		encKey, encNonce, err = f.generateEncryptionKeyAndNonce()
		if err != nil {
			return nil, err
		}

		scannerPrev := scanner
		scanner = func(r io.Reader) ([]string, error) {
			// #nosec G407 -- false positive
			enc, err := NewEnvDecryptor(encKey, encNonce, r)
			if err != nil {
				return nil, err
			}
			return scannerPrev(enc)
		}
	}

	if f.useFifo && runtimestd.GOOS != "windows" {
		return newEnvCollectorFifo(scanner, encKey, encNonce)
	}

	return newEnvCollectorFile(scanner, encKey, encNonce)
}

func (f *EnvCollectorFactory) generateEncryptionKeyAndNonce() ([]byte, []byte, error) {
	key, err := createEnvEncryptionKey()
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to create the encryption key")
	}

	nonce, err := createEnvEncryptionNonce()
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to create the encryption nonce")
	}

	return key, nonce, nil
}
