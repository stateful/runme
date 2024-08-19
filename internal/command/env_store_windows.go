//go:build windows

package command

// MaxEnvironSizeInBytes is the maximum size of an environment variable
// including equal sign and NUL separators.
var MaxEnvironSizeInBytes = 32767
