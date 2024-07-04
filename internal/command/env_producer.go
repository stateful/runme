package command

import (
	"io"
	"os"
	"strings"
)

type EnvProducer struct {
	envStream io.Reader
}

func (p *EnvProducer) Read(b []byte) (n int, err error) {
	if p.envStream == nil {
		p.envStream = strings.NewReader(p.environ())
	}
	return p.envStream.Read(b)
}

func (p *EnvProducer) environ() string {
	return strings.Join(os.Environ(), "\x00")
}
