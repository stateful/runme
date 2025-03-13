package document

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runmedev/runme/v3/internal/renderer/cmark"
	"github.com/runmedev/runme/v3/pkg/document/identity"
)

var allIdentityResolver = identity.NewResolver(identity.AllLifecycleIdentity)

func TestDocument_Parse(t *testing.T) {
	data := []byte(`# Examples

First paragraph.

> bq1
>
>     echo "inside bq"
>
> bq2
> bq3

1. Item 1

   ` + "```" + `sh {name=echo first= second=2}
   $ echo "Hello, runme!"
   ` + "```" + `

   Inner paragraph

2. Item 2
3. Item 3
`)
	doc := New(data, allIdentityResolver)
	node, err := doc.Root()
	require.NoError(t, err)
	assert.Len(t, node.children, 4)
	assert.Len(t, node.children[0].children, 0)
	assert.Len(t, node.children[1].children, 0)
	assert.Len(t, node.children[2].children, 3)
	assert.Equal(t, "> bq1\n>\n>     echo \"inside bq\"\n>\n> bq2\n> bq3\n", string(node.children[2].Item().Value()))
	assert.Len(t, node.children[2].children[0].children, 0)
	assert.Equal(t, "bq1\n", string(node.children[2].children[0].Item().Value()))
	assert.Len(t, node.children[2].children[1].children, 0)
	assert.Equal(t, "```\necho \"inside bq\"\n```\n", string(node.children[2].children[1].Item().Value()))
	assert.Len(t, node.children[2].children[2].children, 0)
	assert.Equal(t, "bq2\nbq3\n", string(node.children[2].children[2].Item().Value()))
	assert.Len(t, node.children[3].children, 3)
	assert.Len(t, node.children[3].children[0].children, 3)
	assert.Equal(t, "1. Item 1\n\n   ```sh {name=echo first= second=2}\n   $ echo \"Hello, runme!\"\n   ```\n\n   Inner paragraph\n", string(node.children[3].children[0].Item().Value()))
	assert.Len(t, node.children[3].children[0].children[0].children, 0)
	assert.Equal(t, "Item 1\n", string(node.children[3].children[0].children[0].Item().Value()))
	assert.Len(t, node.children[3].children[0].children[1].children, 0)
	assert.Equal(t, "```sh {name=echo first= second=2}\n$ echo \"Hello, runme!\"\n```\n", string(node.children[3].children[0].children[1].Item().Value()))
	assert.Len(t, node.children[3].children[0].children[2].children, 0)
	assert.Equal(t, "Inner paragraph\n", string(node.children[3].children[0].children[2].Item().Value()))
	assert.Len(t, node.children[3].children[1].children, 1)
	assert.Equal(t, "2. Item 2\n", string(node.children[3].children[1].Item().Value()))
	assert.Len(t, node.children[3].children[1].children[0].children, 0)
	assert.Equal(t, "Item 2\n", string(node.children[3].children[1].children[0].Item().Value()))
	assert.Len(t, node.children[3].children[2].children, 1)
	assert.Equal(t, "3. Item 3\n", string(node.children[3].children[2].Item().Value()))
	assert.Len(t, node.children[3].children[2].children[0].children, 0)
	assert.Equal(t, "Item 3\n", string(node.children[3].children[2].children[0].Item().Value()))
}

func TestDocument_Frontmatter(t *testing.T) {
	lcidRegex := `---\nkey: value\nrunme:\n  id: .*\n  version: v(?:[3-9]\d*|2\.\d+\.\d+|2\.\d+|\d+)\n---`
	data := bytes.TrimSpace([]byte(`
---
key: value
---

First paragraph
`,
	))

	t.Run("Parse_DefaultIdentity", func(t *testing.T) {
		defaultIdentityResolver := identity.NewResolver(identity.DefaultLifecycleIdentity)
		doc := New(data, defaultIdentityResolver)
		err := doc.Parse()
		require.NoError(t, err)
		assert.Equal(t, []byte("First paragraph"), doc.Content())
		assert.Equal(t, 20, doc.ContentOffset())

		frontmatter, err := doc.FrontmatterWithError()
		require.NoError(t, err)
		marshaledFrontmatter, err := frontmatter.Marshal(defaultIdentityResolver.DocumentEnabled())
		require.NoError(t, err)
		assert.NotRegexp(t, lcidRegex, string(marshaledFrontmatter))
	})

	t.Run("Parse_AllIdentity", func(t *testing.T) {
		doc := New(data, allIdentityResolver)
		err := doc.Parse()
		require.NoError(t, err)
		assert.Equal(t, []byte("First paragraph"), doc.Content())
		assert.Equal(t, 20, doc.ContentOffset())

		frontmatter, err := doc.FrontmatterWithError()
		require.NoError(t, err)
		marshaledFrontmatter, err := frontmatter.Marshal(allIdentityResolver.DocumentEnabled())
		require.NoError(t, err)
		assert.Regexp(t, lcidRegex, string(marshaledFrontmatter))
	})

	t.Run("Format", func(t *testing.T) {
		testCases := []struct {
			Name   string
			Format string
			Data   []byte
		}{
			{
				Name:   "YAML",
				Format: frontmatterFormatYAML,
				Data: bytes.TrimSpace([]byte(`
---
shell: fish
---
				`,
				)),
			},
			{
				Name: "JSON",
				// TODO(adamb): technically, JSON is valid YAML and the lib that is used here
				// does not allow to disable that. Figure out a solution.
				Format: frontmatterFormatYAML,
				Data: bytes.TrimSpace([]byte(`
---
{
  "shell": "fish"
}
---
				`,
				)),
			},
			{
				Name:   "TOML",
				Format: frontmatterFormatTOML,
				Data: bytes.TrimSpace([]byte(`
---
shell = "fish"
---
				`,
				)),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				doc := New(tc.Data, allIdentityResolver)
				_, err := doc.Root()
				require.NoError(t, err)

				frontmatter, err := doc.FrontmatterWithError()
				assert.NoError(t, err)
				assert.Equal(t, "fish", frontmatter.Shell)
			})
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		data := bytes.TrimSpace([]byte(`
---
{
  "shell": "bin/sh",
  "cwd": "/path/to/cwd
}
---
`,
		))
		doc := New(data, allIdentityResolver)
		err := doc.Parse()
		require.NoError(t, err)

		frontmatter, err := doc.FrontmatterWithError()
		assert.ErrorContains(t, err, "failed to parse frontmatter content: yaml: line 3: found unexpected end of stream")
		assert.Nil(t, frontmatter)
	})
}

func TestDocument_TrailingLineBreaks(t *testing.T) {
	data := []byte(`This will test final line breaks`)

	t.Run("No breaks", func(t *testing.T) {
		doc := New(data, allIdentityResolver)
		astNode, err := doc.RootAST()
		require.NoError(t, err)

		actual, err := cmark.Render(astNode, data)
		require.NoError(t, err)

		assert.Equal(
			t,
			string(data),
			string(actual),
		)
		assert.Equal(t, 0, doc.TrailingLineBreaksCount())
	})

	t.Run("1 LF", func(t *testing.T) {
		withLineBreaks := append(data, bytes.Repeat([]byte{'\n'}, 1)...)
		doc := New(withLineBreaks, allIdentityResolver)
		astNode, err := doc.RootAST()
		require.NoError(t, err)

		actual, err := cmark.Render(astNode, withLineBreaks)
		require.NoError(t, err)

		assert.Equal(
			t,
			string(withLineBreaks),
			string(actual),
		)
		assert.Equal(t, 1, doc.TrailingLineBreaksCount())
	})

	t.Run("1 CRLF", func(t *testing.T) {
		withLineBreaks := append(data, bytes.Repeat([]byte{'\r', '\n'}, 1)...)
		doc := New(withLineBreaks, allIdentityResolver)
		astNode, err := doc.RootAST()
		require.NoError(t, err)

		actual, err := cmark.Render(astNode, withLineBreaks)
		require.NoError(t, err)

		assert.Equal(
			t,
			string(withLineBreaks),
			string(actual),
		)
		assert.Equal(t, 1, doc.TrailingLineBreaksCount())
	})

	t.Run("7 LFs", func(t *testing.T) {
		withLineBreaks := append(data, bytes.Repeat([]byte{'\n'}, 7)...)
		doc := New(withLineBreaks, allIdentityResolver)
		astNode, err := doc.RootAST()
		require.NoError(t, err)

		actual, err := cmark.Render(astNode, withLineBreaks)
		require.NoError(t, err)

		assert.Equal(
			t,
			string(actual),
			string(withLineBreaks),
		)
		assert.Equal(t, 7, doc.TrailingLineBreaksCount())
	})
}
