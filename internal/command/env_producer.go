package command

import (
	"encoding/hex"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
)

func NewEnvProducerFromEnv() (*EnvProducer, error) {
	reader := newLazyReader(func() (io.Reader, error) {
		return getEnvironAsReader(), nil
	})

	encKey := os.Getenv(envCollectorEncKeyEnvName)
	encNonce := os.Getenv(envCollectorEncNonceEnvName)

	if encKey != "" && encNonce != "" {
		encKey, err := hex.DecodeString(encKey)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode encryption key")
		}

		encNonce, err := hex.DecodeString(encNonce)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode nonce")
		}

		enc, err := NewEnvEncryptor(encKey, encNonce, reader)
		if err != nil {
			return nil, err
		}
		return &EnvProducer{envStream: enc}, nil
	}

	return &EnvProducer{envStream: reader}, nil
}

type EnvProducer struct {
	envStream io.Reader
}

func (p *EnvProducer) Read(b []byte) (n int, err error) {
	if p.envStream == nil {
		p.envStream = strings.NewReader(getEnvironAsString())
	}
	return p.envStream.Read(b)
}

func EnvSliceToString(env []string) string {
	return strings.Join(env, "\x00")
}

func getEnvironAsReader() io.Reader {
	return strings.NewReader(getEnvironAsString())
}

func getEnvironAsString() string {
	return EnvSliceToString(os.Environ())
}

func newLazyReader(init func() (io.Reader, error)) *lazyReader {
	return &lazyReader{init: init}
}

type lazyReader struct {
	init func() (io.Reader, error)
	r    io.Reader
	err  error
}

func (r *lazyReader) Read(p []byte) (n int, err error) {
	if r.r == nil {
		r.r, r.err = r.init()
		if r.err != nil {
			return 0, r.err
		}
	}
	if r.err != nil {
		return 0, r.err
	}
	return r.r.Read(p)
}
