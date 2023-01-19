//go:build !windows
// +build !windows

package socket

import (
	"net"
)

func ListenPipe(path string) (net.Listener, error) {
	return net.Listen("unix", path)
}
