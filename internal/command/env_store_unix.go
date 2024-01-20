//go:build !windows

package command

// MaxEnvironSizInBytes is the maximum size of an environment variable
// including equal sign and NUL separators.
//
// This size is an artificial limit as Linux and macOS do not have a real limit.
const MaxEnvironSizInBytes = 128 * 1000 * 1000 // 128 MB
