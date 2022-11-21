package main

import (
	"syscall/js"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeserialize(t *testing.T) {
	source := js.ValueOf([]byte(`# Hi!\n\nSome paragraph.\n`))
	result := deserialize(js.Null(), []js.Value{source})
	assert.Equal(t, ``, result)
}
