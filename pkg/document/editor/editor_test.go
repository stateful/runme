package editor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stateful/runme/v3/internal/ulid"
	"github.com/stateful/runme/v3/internal/version"
	"github.com/stateful/runme/v3/pkg/document"
	"github.com/stateful/runme/v3/pkg/document/constants"
	"github.com/stateful/runme/v3/pkg/document/identity"
)

var (
	identityResolverNone = identity.NewResolver(identity.UnspecifiedLifecycleIdentity)
	identityResolverAll  = identity.NewResolver(identity.AllLifecycleIdentity)
	identityResolverCell = identity.NewResolver(identity.CellLifecycleIdentity)
	identityResolverDoc  = identity.NewResolver(identity.DocumentLifecycleIdentity)
	testMockID           = ulid.GenerateID()
)

func TestMain(m *testing.M) {
	ulid.MockGenerator(testMockID)
	code := m.Run()
	ulid.ResetGenerator()
	os.Exit(code)
}

func TestEditor(t *testing.T) {
	notebook, err := Deserialize(testDataNested, Options{IdentityResolver: identityResolverNone})
	require.NoError(t, err)
	result, err := Serialize(notebook, nil, Options{})
	require.NoError(t, err)
	assert.Equal(
		t,
		string(testDataNestedFlattened),
		string(result),
	)
}

func TestEditor_List(t *testing.T) {
	data := []byte(`1. Item 1
2. Item 2
3. Item 3
`)
	notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
	require.NoError(t, err)

	notebook.Cells[0].Value = "1. Item 1\n2. Item 2\n"

	newData, err := Serialize(notebook, nil, Options{})
	require.NoError(t, err)
	assert.Equal(
		t,
		`1. Item 1
2. Item 2
`,
		string(newData),
	)

	newData, err = Serialize(notebook, nil, Options{})
	require.NoError(t, err)
	assert.Equal(
		t,
		`1. Item 1
2. Item 2
`,
		string(newData),
	)
}

func TestEditor_CodeBlock(t *testing.T) {
	t.Run("ProvideGeneratedName", func(t *testing.T) {
		data := []byte("```sh\necho 1\n```\n")
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)
		cell := notebook.Cells[0]
		assert.Equal(
			t,
			cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "name")],
			"echo-1",
		)
		assert.Equal(
			t,
			cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "nameGenerated")],
			"true",
		)
		// "name" is empty because it was not included in the original snippet.
		assert.Empty(
			t,
			cell.Metadata["name"],
		)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(t, string(data), string(result))
	})

	t.Run("PreserveName", func(t *testing.T) {
		data := []byte("```sh {\"name\":\"name1\"}\necho 1\n```\n")
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)
		cell := notebook.Cells[0]
		assert.Equal(
			t,
			cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "name")],
			"name1",
		)
		assert.Equal(
			t,
			cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "nameGenerated")],
			"false",
		)
		// "name" is not nil because it was included in the original snippet.
		assert.Equal(
			t,
			cell.Metadata["name"],
			"name1",
		)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(t, string(data), string(result))
	})
}

func TestEditor_CellLifecycleIdentity(t *testing.T) {
	t.Run("NoIdentity", func(t *testing.T) {
		notebook, err := Deserialize(testDataNested, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		for _, cell := range notebook.Cells {
			if cell.Kind != CellKind(document.CodeBlockKind) {
				continue
			}
			assert.Empty(t, cell.Metadata["id"])
			assert.NotEmpty(t, cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "id")])
		}
	})

	t.Run("CellIdentity", func(t *testing.T) {
		notebook, err := Deserialize(testDataNested, Options{IdentityResolver: identityResolverCell})
		require.NoError(t, err)

		for _, cell := range notebook.Cells {
			if cell.Kind != CellKind(document.CodeBlockKind) {
				continue
			}
			assert.NotEmpty(t, cell.Metadata["id"])
			assert.NotEmpty(t, cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "id")])
			assert.Equal(t, cell.Metadata["id"], cell.Metadata[PrefixAttributeName(InternalAttributePrefix, "id")])
		}
	})
}

func TestEditor_Metadata(t *testing.T) {
	data := []byte(`# Heading Level 1
Paragraph 1 with a link [Link1](https://example.com 'Link Title 1') and a second link [Link2](https://example2.com 'Link Title 2')..
## Heading Level 2
### Heading Level 3
#### Heading Level 4
##### Heading Level 5
`)
	err := os.Setenv("RUNME_AST_METADATA", "true")
	require.NoError(t, err)

	notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
	require.NoError(t, err)
	require.NotEmpty(t, notebook.Metadata)

	astStr := notebook.Cells[0].Metadata["runme.dev/ast"]
	var metadata map[string]interface{}
	err = json.Unmarshal([]byte(astStr), &metadata)
	require.NoError(t, err)

	assert.Equal(t, "Heading", metadata["Kind"])
	assert.Equal(t, 1, len(metadata["Children"].([]interface{})))
	assert.Equal(t, 1, int(metadata["Level"].(float64)))

	astStr = notebook.Cells[1].Metadata["runme.dev/ast"]
	metadata = make(map[string]interface{})
	err = json.Unmarshal([]byte(astStr), &metadata)
	require.NoError(t, err)

	assert.Equal(t, "Paragraph", metadata["Kind"])
	assert.Equal(t, 5, len(metadata["Children"].([]interface{})))

	children := metadata["Children"].([]interface{})

	nChild := children[0].(map[string]interface{})
	assert.Equal(t, "Paragraph 1 with a link ", nChild["Text"].(string))

	nChild = children[1].(map[string]interface{})
	assert.Equal(t, "https://example.com", nChild["Destination"].(string))
	assert.Equal(t, "Link Title 1", nChild["Title"].(string))

	children = nChild["Children"].([]interface{})
	nChild = children[0].(map[string]interface{})
	assert.Equal(t, "Link1", nChild["Text"].(string))
}

func TestEditor_Frontmatter(t *testing.T) {
	data := []byte(fmt.Sprintf(`+++
prop1 = 'val1'
prop2 = 'val2'

[runme]
id = '%s'
version = '%s'
+++

# Example

A paragraph
`, testMockID, version.BaseVersion()))
	notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
	require.NoError(t, err)
	result, err := Serialize(notebook, nil, Options{})
	require.NoError(t, err)
	assert.Equal(
		t,
		string(data),
		string(result),
	)
}

func TestEditor_FrontmatterWithoutRunme(t *testing.T) {
	data := []byte(`+++
prop1 = 'val1'
prop2 = 'val2'
+++

# Example

A paragraph
`)
	notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
	require.NoError(t, err)
	result, err := Serialize(notebook, nil, Options{})
	require.NoError(t, err)
	assert.Equal(
		t,
		string(data),
		string(result),
	)
}

func TestEditor_RetainInvalidFrontmatter(t *testing.T) {
	data := []byte(`+++
title = '{{ replace .File.ContentBaseName "-" " " | title }}'
date = {{ .Date }}
draft = true
+++

# Example

A paragraph
`)
	notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
	require.NoError(t, err)
	result, err := Serialize(notebook, nil, Options{})
	require.NoError(t, err)
	assert.Equal(
		t,
		string(data),
		string(result),
	)
}

func TestEditor_SessionOutput(t *testing.T) {
	data := []byte(fmt.Sprintf(`+++
prop1 = 'val1'
prop2 = 'val2'

[runme]
id = '%s'
version = '%s'
+++

# Example

A paragraph
`, testMockID, version.BaseVersion()))
	notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
	require.NoError(t, err)

	sid := "01HJP23P1R57BPGEA17QDJXJE"
	rpath := "README.md"
	invalidTs := "invalid-timestamp-should-be-overwritten"
	outputMetadata := &document.RunmeMetadata{
		Session: &document.RunmeMetadataSession{
			ID:      sid,
			Updated: invalidTs,
		},
		Document: &document.RunmeMetadataDocument{RelativePath: rpath},
	}
	result, err := Serialize(notebook, outputMetadata, Options{})
	require.NoError(t, err)
	assert.Contains(
		t,
		string(result),
		string(sid),
	)

	sessionNb, err := Deserialize(result, Options{IdentityResolver: identityResolverAll})
	require.NoError(t, err)

	sess := sessionNb.Frontmatter.Runme.Session
	assert.Equal(t, sid, sess.ID)
	assert.NotEqual(t, sess.Updated, invalidTs)
	assert.Greater(t, len(sess.Updated), 0)

	doc := sessionNb.Frontmatter.Runme.Document
	assert.Equal(t, doc.RelativePath, rpath)
}

func TestEditor_Newlines(t *testing.T) {
	data := []byte(`## Newline debugging

This will test final line breaks`)

	t.Run("No line breaks", func(t *testing.T) {
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.Equal(
			t,
			notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey)],
			"0",
		)

		actual, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(data),
			string(actual),
		)
	})

	t.Run("Single line break", func(t *testing.T) {
		withLineBreaks := append(data, byte('\n'))

		notebook, err := Deserialize(withLineBreaks, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.Equal(
			t,
			notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey)],
			"1",
		)

		actual, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(withLineBreaks),
			string(actual),
		)
	})

	t.Run("7 line breaks", func(t *testing.T) {
		withLineBreaks := append(data, bytes.Repeat([]byte{'\n'}, 7)...)

		notebook, err := Deserialize(withLineBreaks, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.Equal(
			t,
			notebook.Metadata[PrefixAttributeName(InternalAttributePrefix, constants.FinalLineBreaksKey)],
			"7",
		)

		actual, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(withLineBreaks),
			string(actual),
		)
	})
}

func TestEditor_ResetIdentity(t *testing.T) {
	codeCell := "```sh {\"id\":\"abcdefg\"}\necho 1\n```"
	runmeYamlFm := fmt.Sprintf(`runme:
  id: %s
  version: %s`, testMockID, version.BaseVersion())
	data := []byte(fmt.Sprintf(`---
prop1: val1
prop2: val2
%s
---

# Example

A paragraph

%s
`, runmeYamlFm, codeCell))

	t.Run("WithoutResetNoIdentity", func(t *testing.T) {
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone, Reset: false})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(data),
			string(result),
		)
	})

	t.Run("WithResetNoIdentity", func(t *testing.T) {
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone, Reset: true})
		require.NoError(t, err)

		// notebook-level
		require.NotNil(t, notebook.Metadata["runme.dev/cache-id"])
		require.Empty(t, notebook.Metadata["id"])
		require.EqualValues(t, "---\nprop1: val1\nprop2: val2\n---", notebook.Metadata["runme.dev/frontmatter"])

		// cell-level
		cell := notebook.Cells[2]
		require.EqualValues(t, 2, cell.Kind)
		require.EqualValues(t, "echo 1", cell.Value)
		require.EqualValues(t, "sh", cell.LanguageID)
		require.Empty(t, cell.Metadata["id"])
	})

	t.Run("WithResetWithIdentity", func(t *testing.T) {
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverDoc, Reset: true})
		require.NoError(t, err)

		// notebook-level
		require.NotNil(t, notebook.Metadata["runme.dev/cache-id"])
		require.Empty(t, notebook.Metadata["id"])
		expected := fmt.Sprintf("---\nprop1: val1\nprop2: val2\nrunme:\n  id: %s\n  version: %s\n---", testMockID, version.BaseVersion())
		require.EqualValues(t, expected, notebook.Metadata["runme.dev/frontmatter"])

		// cell-level
		cell := notebook.Cells[2]
		require.EqualValues(t, 2, cell.Kind)
		require.EqualValues(t, "echo 1", cell.Value)
		require.EqualValues(t, "sh", cell.LanguageID)
		require.Empty(t, cell.Metadata["id"])
	})
}

func TestEditor_ResetRunmeFrontmatterYAML(t *testing.T) {
	t.Run("RetainUnrelated", func(t *testing.T) {
		data := []byte(`---
runme:
  id: 01HFVTDYA775K2HREH9ZGQJ75B
  version: v3
unrelated: frontmatter
---

# Example

A paragraph
`)
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone, Reset: true})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			"---\nunrelated: frontmatter\n---\n\n# Example\n\nA paragraph\n",
			string(result),
		)
	})

	t.Run("RemoveEmpty", func(t *testing.T) {
		data := []byte(`---
runme:
  id: 01HFVTDYA775K2HREH9ZGQJ75B
  version: v3
---

# Example

A paragraph
`)
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone, Reset: true})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			"# Example\n\nA paragraph\n",
			string(result),
		)
	})
}

func TestEditor_ResetRunmeFrontmatterJSON(t *testing.T) {
	t.Run("RetainUnrelated", func(t *testing.T) {
		data := []byte(`{
  "runme": {
    "id": "01HF7YYYYYYYYYYYYMQ2KEEYGM",
    "version": "v3"
  },
  "unrelated": "frontmatter"
}

# Example

A paragraph
`)
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone, Reset: true})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			"{\n  \"unrelated\": \"frontmatter\"\n}\n\n# Example\n\nA paragraph\n",
			string(result),
		)
	})

	t.Run("RemoveEmpty", func(t *testing.T) {
		data := []byte(`{
  "runme": {
    "id": "01HF7YYYYYYYYYYYYMQ2KEEYGM",
    "version": "v3"
  }
}

# Example

A paragraph
`)
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone, Reset: true})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			"# Example\n\nA paragraph\n",
			string(result),
		)
	})
}

func TestEditor_ResetRunmeFrontmatterTOML(t *testing.T) {
	t.Run("RetainUnrelated", func(t *testing.T) {
		data := []byte(`+++
unrelated = 'frontmatter'
[runme]
id = '01HRA297WC2XXXXXX8FM3DR1V0'
version = 'v3'
+++

# Example

A paragraph
`)
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone, Reset: true})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			"+++\nunrelated = 'frontmatter'\n+++\n\n# Example\n\nA paragraph\n",
			string(result),
		)
	})

	t.Run("RemoveEmpty", func(t *testing.T) {
		data := []byte(`+++
[runme]
id = '01HRA297WC2XXXXXX8FM3DR1V0'
version = 'v3'
+++

# Example

A paragraph
`)
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone, Reset: true})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			"# Example\n\nA paragraph\n",
			string(result),
		)
	})
}

func TestEditor_CodeBlockTransformation(t *testing.T) {
	t.Run("MermaidException", func(t *testing.T) {
		data := []byte("```mermaid\ngraph TD;\nA-->B;\nB-->C;\n```")
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.EqualValues(t, MarkupKind, notebook.Cells[0].Kind)

		newData, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(data),
			string(newData),
		)
	})

	t.Run("MermaidForceTransformation", func(t *testing.T) {
		data := []byte("```mermaid {\"transform\":\"true\"}\ngraph TD;\nA-->B;\nB-->C;\n```")
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.EqualValues(t, CodeKind, notebook.Cells[0].Kind)

		newData, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(data),
			string(newData),
		)
	})

	t.Run("UnsetDefault", func(t *testing.T) {
		data := []byte("```javascript\nconsole.log('test');\n```")
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.EqualValues(t, CodeKind, notebook.Cells[0].Kind)

		newData, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(data),
			string(newData),
		)
	})

	t.Run("InvalidUsesDefault", func(t *testing.T) {
		data := []byte("```javascript {\"transform\":\"abcdefg\"}\nconsole.log('test');\n```")
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.EqualValues(t, CodeKind, notebook.Cells[0].Kind)

		newData, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(data),
			string(newData),
		)
	})

	t.Run("False", func(t *testing.T) {
		data := []byte("```javascript {\"transform\":\"false\"}\nconsole.log('test');\n```")
		notebook, err := Deserialize(data, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.EqualValues(t, MarkupKind, notebook.Cells[0].Kind)

		newData, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(data),
			string(newData),
		)
	})
}

func TestEditor_CodeBlockEndcoding(t *testing.T) {
	t.Parallel()

	t.Run("Fenced", func(t *testing.T) {
		original := []byte(`## Tests

Run all tests:

` + strings.Repeat("`", 3) + `sh
export TESTING="1"
dagger call test all
` + strings.Repeat("`", 3) + `

Run a specific test:

` + strings.Repeat("`", 3) + `sh
dagger call test specific --pkg="./core/integration" --run="^TestModule/TestNamespacing$"
` + strings.Repeat("`", 3) + `

A paragraph
`)
		notebook, err := Deserialize(original, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(original),
			string(result),
		)
	})

	t.Run("UnfencedWithSpaces", func(t *testing.T) {
		original := []byte(`## Tests

Run all tests:

    export TESTING="1"
    dagger call test all

Run a specific test:

    dagger call test specific --pkg="./core/integration" --run="^TestModule/TestNamespacing$"

A paragraph
`)
		notebook, err := Deserialize(original, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)
		result, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)
		assert.Equal(
			t,
			string(original),
			string(result),
		)
	})
}

func TestEditor_AttributeFormat(t *testing.T) {
	t.Run("RetainOriginalFormat", func(t *testing.T) {
		mixedFormats := []byte("## Retain attribute format\n\nFirst block uses HTML.\n\n```sh { name=date interactive=false }\ndate\n```\n\nThe second JSON.\n\n```javascript {\"interactive\":\"false\",\"name\":\"iso\"}\nconsole.log(new Date().toISOString())\n```\n")

		notebook, err := Deserialize(mixedFormats, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		actual, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)

		assert.Equal(
			t,
			string(mixedFormats),
			string(actual),
		)
	})
}

func TestEditor_LabelComments(t *testing.T) {
	labelComments := []byte("## Retain attribute format\n\nFirst block uses HTML.\n\n```sh { name=date interactive=false }\n### Exported in runme.dev as date\ndate\n```\n\nThe second JSON.\n\n```javascript {\"interactive\":\"false\",\"name\":\"iso\"}\n### Exported in runme.dev as iso\nconsole.log(new Date().toISOString())\n```\n")
	t.Run("StrippedByDefault", func(t *testing.T) {

		notebook, err := Deserialize(labelComments, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.Nil(t, notebook.Frontmatter)

		assert.Equal(t, "date", notebook.Cells[2].Metadata["name"])
		require.NotContains(t, notebook.Cells[2].Value, labelCommentPreamble)

		assert.Equal(t, "iso", notebook.Cells[4].Metadata["name"])
		require.NotContains(t, notebook.Cells[4].Value, labelCommentPreamble)

		actual, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)

		require.NotContains(t, string(actual), labelCommentPreamble+"date")
		require.NotContains(t, string(actual), labelCommentPreamble+"iso")
	})

	t.Run("SerializeForDaggerShell", func(t *testing.T) {
		withFrontmatter := bytes.Join([][]byte{[]byte("---\nshell: dagger shell\n---\n\n"), labelComments}, nil)

		notebook, err := Deserialize(withFrontmatter, Options{IdentityResolver: identityResolverNone})
		require.NoError(t, err)

		assert.Equal(t, "dagger shell", notebook.Frontmatter.Shell)

		assert.Equal(t, "date", notebook.Cells[2].Metadata["name"])
		require.NotContains(t, notebook.Cells[2].Value, labelCommentPreamble)

		assert.Equal(t, "iso", notebook.Cells[4].Metadata["name"])
		require.NotContains(t, notebook.Cells[4].Value, labelCommentPreamble)

		actual, err := Serialize(notebook, nil, Options{})
		require.NoError(t, err)

		require.Contains(t, string(actual), labelCommentPreamble+"date")
		require.Contains(t, string(actual), labelCommentPreamble+"iso")
	})
}
