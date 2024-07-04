package command

import (
	"io"

	"github.com/pkg/errors"
)

func newFifoShellEnvCollector(io.Writer) (shellEnvCollector, error) {
	return nil, errors.Wrap(errFifoCreate, "fifo unsupported")
}
