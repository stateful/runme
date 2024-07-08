//go:build windows
// +build windows

package command

import "github.com/pkg/errors"

func newEnvCollectorFifo(envScanner, []byte, []byte) (envCollector, error) {
	return nil, errors.New("not implemented")
}
