package command

import (
	"io"
	runtimestd "runtime"

	"github.com/pkg/errors"
)

type envCollectorFactoryOptions struct {
	encryptionEnabled bool
	useFifo           bool
}

type envCollectorFactory struct {
	opts envCollectorFactoryOptions
}

func newEnvCollectorFactory(opts envCollectorFactoryOptions) *envCollectorFactory {
	return &envCollectorFactory{
		opts: opts,
	}
}

func (f *envCollectorFactory) Build() (envCollector, error) {
	scanner := scanEnv

	var (
		encKey   []byte
		encNonce []byte
	)

	if f.opts.encryptionEnabled {
		var err error

		encKey, encNonce, err = f.generateEncryptionKeyAndNonce()
		if err != nil {
			return nil, err
		}

		scannerPrev := scanner
		scanner = func(r io.Reader) ([]string, error) {
			enc, err := NewEnvDecryptor(encKey, encNonce, r)
			if err != nil {
				return nil, err
			}
			return scannerPrev(enc)
		}
	}

	if f.opts.useFifo && runtimestd.GOOS != "windows" {
		return newEnvCollectorFifo(scanner, encKey, encNonce)
	}

	return newEnvCollectorFile(scanner, encKey, encNonce)
}

func (f *envCollectorFactory) generateEncryptionKeyAndNonce() ([]byte, []byte, error) {
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
