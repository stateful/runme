package cmd

import (
	"bytes"
	"regexp"

	"github.com/stateful/runme/internal/extension"
)

func swapExtensioner(ext extension.Extensioner) func() {
	extension := extension.Default()
	prev := extension
	extension = ext
	return func() {
		extension = prev
	}
}

type cleanBuffer struct {
	*bytes.Buffer
}

func (b *cleanBuffer) String() string {
	return removeEscapes(b.Buffer.String())
}

func removeEscapes(in string) string {
	re := regexp.MustCompile(`(?:\x1B[@-Z\\-_]|[\x80-\x9A\x9C-\x9F]|(?:\x1B\[|\x9B)[0-?]*[ -/]*[@-~])`)
	return re.ReplaceAllString(in, "")
}
