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

	// todo(sebastian): perhaps it make sense to write this message at the TTY-level?
	termInitMessage := ("# Runme: This terminal forked your session. " +
		"Upon exit exported environment variables will be rolled up into the session.\n\n")

	if f.opts.useFifo && runtimestd.GOOS != "windows" {
		return newEnvCollectorFifo(scanner, termInitMessage, encKey, encNonce)
	}

	return newEnvCollectorFile(scanner, termInitMessage, encKey, encNonce)
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
