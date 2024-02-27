package cmark_test

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stateful/runme/v3/internal/renderer/cmark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

func testEquality(t *testing.T, data []byte) {
	parser := goldmark.DefaultParser()
	ast := parser.Parse(text.NewReader(data))
	result, err := cmark.Render(ast, data)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(result))
}

func TestRender_HTMLBlock(t *testing.T) {
	data := []byte(`---

<p align="center"><small>Copyright 2022 © <a href="https://stateful.com/">Stateful</a> (<a href="https://discord.gg/BQm8zRCBUY">💬 Join Discord</a>) – Apache 2.0 License</small> </p>
`)
	testEquality(t, data)
}

func TestRender_TightList(t *testing.T) {
	data := []byte(`List example:

1. Item 1
2. Item 2
3. Item 3
`)
	testEquality(t, data)
}

func TestRender_List_Marker(t *testing.T) {
	data := []byte(`## Prerequisites

* Tutorial was done on macOS.
* Tutorial assumes you have Homebrew installed on you computer. If not, you can install here: https://brew.sh
* Tutorial assumes you have Docker installed on your computer. If not, you can install it here: https://docs.docker.com/docker-for-mac/install/
`)
	testEquality(t, data)
}

func TestRender_ListWithCodeBlock(t *testing.T) {
	data := []byte(`1. **Clone this repository.**

` + "```" + `
git clone https://github.com/my/repo.git
cd my-repo
` + "```" + `

2. **Create a cluster.**

- Autopilot mode:

` + "```" + `
REGION=us-central1
cluster-create
` + "```" + `

- Standard mode:

` + "```" + `
REGION=us-central1
cluster-create-std
` + "```" + `

4. **Deploy the sample app to the cluster.**
`)

	testEquality(t, data)
}

func TestRender_FencedCodeBlockAttributes(t *testing.T) {
	data := []byte("```sh {name=echo first= second=2}\necho 1\n```\n")
	testEquality(t, data)
}

func TestRender_Testdata(t *testing.T) {
	err := filepath.Walk("../testdata", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			f, err := os.Open(path)
			require.NoError(t, err)
			data, err := io.ReadAll(f)
			require.NoError(t, err)
			testEquality(t, data)
		})
		return nil
	})
	require.NoError(t, err)
}
