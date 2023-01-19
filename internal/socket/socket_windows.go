package socket

import (
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

func ListenPipe(path string) (net.Listener, error) {
	return winio.ListenPipe(fixPath(path), nil)
}

// fixPath adjust unix-like path to be compatible with Windows pipe names.
// It does not implement any standard but follows node-ipc package:
// https://github.com/RIAEvangelist/node-ipc/blob/7da90e18f9bf3e7154e22eb86b82cf4a4d5cbc37/dao/socketServer.js#L302-L306
//
// More on Window pipe names:
// https://docs.microsoft.com/en-us/windows/win32/ipc/pipe-names
func fixPath(path string) string {
	path = strings.TrimLeft(path, "/")
	path = strings.ReplaceAll(path, "/", "-")
	return `\\.\pipe\` + path
}
