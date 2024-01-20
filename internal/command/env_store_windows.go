//go:build windows

package command

// MaxEnvironSizInBytes is the maximum size of an environment variable
// including equal sign and NUL separators.
const MaxEnvironSizInBytes = 32767
