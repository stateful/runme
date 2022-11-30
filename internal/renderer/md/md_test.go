package md

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

func TestRender_HTMLBlock(t *testing.T) {
	data := []byte(`---

<p align="center"><small>Copyright 2022 © <a href="https://stateful.com/">Stateful</a> (<a href="https://discord.gg/BQm8zRCBUY">💬 Join Discord</a>) – Apache 2.0 License</small> </p>
`)

	parser := goldmark.DefaultParser()
	ast := parser.Parse(text.NewReader(data))
	result, err := Render(ast, data)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(result))
}

func TestRender_TightList(t *testing.T) {
	data := []byte(`List example:

1. Item 1
2. Item 2
3. Item 3
`)

	parser := goldmark.DefaultParser()
	ast := parser.Parse(text.NewReader(data))
	result, err := Render(ast, data)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(result))
}

func TestRender_List_Marker(t *testing.T) {
	data := []byte(`## Prerequisites

* Tutorial was done on macOS.
* Tutorial assumes you have Homebrew installed on you computer. If not, you can install here: https://brew.sh
* Tutorial assumes you have Docker installed on your computer. If not, you can install it here: https://docs.docker.com/docker-for-mac/install/
`)
	parser := goldmark.DefaultParser()
	ast := parser.Parse(text.NewReader(data))
	result, err := Render(ast, data)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(result))
}
