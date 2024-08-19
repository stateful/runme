//go:build !windows

package command

// MaxEnvironSizeInBytes is the maximum size of an environment variable
// including equal sign and NUL separators.
//
// This size is an artificial limit as Linux and macOS do not have a real limit.
var MaxEnvironSizeInBytes = 128 * 1000 * 1000 // 128 MB
