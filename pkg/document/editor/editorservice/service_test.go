package editorservice

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/runmedev/runme/v3/internal/testutils"
	"github.com/runmedev/runme/v3/internal/ulid"
	"github.com/runmedev/runme/v3/internal/version"
	parserv1 "github.com/runmedev/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
)

var (
	versionRegex = `version: v(?:[3-9]\d*|2\.\d+\.\d+|2\.\d+|\d+)`
	testMockID   = ulid.GenerateID()

	client parserv1.ParserServiceClient

	documentWithoutFrontmatter = strings.Join([]string{
		"# H1",
		"```sh { name=foo id=123 }",
		`echo "Foo"`,
		"```",
		"## H2",
		"```sh { \"name\": \"bar\" }",
		`echo "Bar"`,
		"```",
		"### H3",
		"```js",
		`echo "Shebang"`,
		"```",
	}, "\n")

	documentWithFrontmatter = strings.Join([]string{
		"---",
		"prop: value",
		"runme:",
		"  id: 123",
		"  version: v99",
		"---",
		"",
		documentWithoutFrontmatter,
	}, "\n")
)

func TestMain(m *testing.M) {
	ulid.MockGenerator(testMockID)

	lis := bufconn.Listen(2048)
	server := grpc.NewServer()
	parserv1.RegisterParserServiceServer(server, NewParserServiceServer(zap.NewNop()))
	go server.Serve(lis)

	_, client = testutils.NewGRPCClient(lis, parserv1.NewParserServiceClient)

	code := m.Run()
	ulid.ResetGenerator()
	os.Exit(code)
}

func Test_IdentityUnspecified(t *testing.T) {
	tests := []struct {
		content             string
		hasExtraFrontmatter bool
	}{
		{content: documentWithFrontmatter, hasExtraFrontmatter: true},
		{content: documentWithoutFrontmatter, hasExtraFrontmatter: false},
	}

	for _, tt := range tests {
		identity := parserv1.RunmeIdentity_RUNME_IDENTITY_UNSPECIFIED

		dResp, err := deserialize(client, tt.content, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]
		if tt.hasExtraFrontmatter {
			assert.True(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 4)
			assert.Contains(t, rawFrontmatter, "prop: value\n")
			assert.Contains(t, rawFrontmatter, "id: \"123\"\n")
			assert.Contains(t, rawFrontmatter, "version: v")
		} else {
			assert.False(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 3)
		}

		sResp, err := serializeWithoutOutputs(client, dResp.Notebook)
		assert.NoError(t, err)
		content := string(sResp.Result)

		if tt.hasExtraFrontmatter {
			assert.Regexp(t, "^---\n", content)
		} else {
			assert.NotRegexp(t, "^---\n", content)
			assert.NotRegexp(t, "^\n\n", content)
		}

		assert.Contains(t, content, "```sh { name=foo id=123 }\n")
	}
}

func Test_IdentityAll(t *testing.T) {
	tests := []struct {
		content             string
		hasExtraFrontmatter bool
	}{
		{content: documentWithFrontmatter, hasExtraFrontmatter: true},
		{content: documentWithoutFrontmatter, hasExtraFrontmatter: false},
	}

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_ALL

	for _, tt := range tests {
		dResp, err := deserialize(client, tt.content, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]
		assert.True(t, ok)

		assert.Len(t, dResp.Notebook.Metadata, 4)

		if tt.hasExtraFrontmatter {
			assert.Contains(t, rawFrontmatter, "prop: value")
		}

		assert.Contains(t, rawFrontmatter, "id: "+testMockID)
		assert.Contains(t, rawFrontmatter, "version: "+version.BaseVersion())

		sResp, err := serializeWithoutOutputs(client, dResp.Notebook)
		assert.NoError(t, err)

		content := string(sResp.Result)
		assert.Regexp(t, "^---\n", content)
		assert.Contains(t, content, "runme:\n")
		assert.Contains(t, content, "id: "+testMockID)
		assert.Contains(t, content, "version: "+version.BaseVersion())
		assert.Contains(t, content, "```sh { name=foo id=123 }\n")
		assert.Contains(t, content, "```sh {\"id\":\""+testMockID+"\",\"name\":\"bar\"}\n")
		assert.Contains(t, content, "```js {\"id\":\""+testMockID+"\"}\n")
	}
}

func Test_IdentityDocument(t *testing.T) {
	tests := []struct {
		content             string
		hasExtraFrontmatter bool
	}{
		{content: documentWithFrontmatter, hasExtraFrontmatter: true},
		{content: documentWithoutFrontmatter, hasExtraFrontmatter: false},
	}

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_DOCUMENT

	for _, tt := range tests {
		dResp, err := deserialize(client, tt.content, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]
		assert.True(t, ok)

		assert.Len(t, dResp.Notebook.Metadata, 4)

		if tt.hasExtraFrontmatter {
			assert.Contains(t, rawFrontmatter, "prop: value")
		}

		assert.Contains(t, rawFrontmatter, "id: "+testMockID)
		assert.Regexpf(t, versionRegex, rawFrontmatter, "Wrong version")

		sResp, err := serializeWithoutOutputs(client, dResp.Notebook)
		assert.NoError(t, err)

		content := string(sResp.Result)
		assert.Regexp(t, "^---\n", content)
		assert.Contains(t, content, "runme:\n")
		assert.Contains(t, content, "id: "+testMockID+"\n")
		assert.Contains(t, content, "version: "+version.BaseVersion()+"\n")
		assert.Contains(t, content, "```sh { name=foo id=123 }\n")
		assert.Contains(t, content, "```sh {\"name\":\"bar\"}\n")
		assert.Contains(t, content, "```js\n")
	}
}

func Test_IdentityCell(t *testing.T) {
	tests := []struct {
		content             string
		hasExtraFrontmatter bool
	}{
		{content: documentWithFrontmatter, hasExtraFrontmatter: true},
		{content: documentWithoutFrontmatter, hasExtraFrontmatter: false},
	}

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_CELL

	for _, tt := range tests {
		dResp, err := deserialize(client, tt.content, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]

		if tt.hasExtraFrontmatter {
			assert.True(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 4)
			assert.Contains(t, rawFrontmatter, "prop: value\n")
			assert.Contains(t, rawFrontmatter, "id: \"123\"\n")
			assert.Regexp(t, versionRegex, rawFrontmatter, "Wrong version")
		} else {
			assert.False(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 3)
		}

		sResp, err := serializeWithoutOutputs(client, dResp.Notebook)
		assert.NoError(t, err)

		content := string(sResp.Result)

		if tt.hasExtraFrontmatter {
			assert.Contains(t, content, "runme:\n")
			assert.NotContains(t, content, "id: \"123\"\n")
			assert.Contains(t, content, "version: v")
		} else {
			assert.NotRegexp(t, "^---\n", content)
			assert.NotRegexp(t, "^\n\n", content)
		}

		assert.Contains(t, content, "```sh { name=foo id=123 }\n")
		assert.Contains(t, content, "```sh {\"id\":\""+testMockID+"\",\"name\":\"bar\"}\n")
		assert.Contains(t, content, "```js {\"id\":\""+testMockID+"\"}\n")
	}
}

func Test_RunmelessFrontmatter(t *testing.T) {
	doc := strings.Join([]string{
		"---",
		"prop: value",
		"---",
		"",
		documentWithoutFrontmatter,
	}, "\n")

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_CELL

	dResp, err := deserialize(client, doc, identity)
	assert.NoError(t, err)

	rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]

	assert.True(t, ok)
	assert.Len(t, dResp.Notebook.Metadata, 4)
	assert.Contains(t, rawFrontmatter, "prop: value\n")
	assert.NotContains(t, rawFrontmatter, "id: \"123\"\n")
	assert.NotRegexp(t, versionRegex, rawFrontmatter, "Wrong version")

	sResp, err := serializeWithoutOutputs(client, dResp.Notebook)
	assert.NoError(t, err)

	content := string(sResp.Result)

	assert.NotContains(t, content, "runme:\n")
	assert.NotContains(t, content, "id: \"123\"\n")
	assert.NotContains(t, content, "version: v")
	assert.Contains(t, content, "prop: value\n")
	assert.Regexp(t, "^---\n", content)
	assert.NotRegexp(t, "^\n\n", content)

	assert.Contains(t, content, "```sh { name=foo id=123 }\n")
	assert.Contains(t, content, "```sh {\"id\":\""+testMockID+"\",\"name\":\"bar\"}\n")
	assert.Contains(t, content, "```js {\"id\":\""+testMockID+"\"}\n")
}

func Test_RetainInvalidFrontmatter(t *testing.T) {
	doc := strings.Join([]string{
		"+++",
		"invalid frontmatter",
		"+++",
		"",
		documentWithoutFrontmatter,
	}, "\n")

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_ALL

	dResp, err := deserialize(client, doc, identity)
	assert.NoError(t, err)

	rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]

	assert.True(t, ok)
	assert.Len(t, dResp.Notebook.Metadata, 4)
	assert.Contains(t, rawFrontmatter, "invalid frontmatter")
	assert.NotContains(t, rawFrontmatter, "id: ")
	assert.NotRegexp(t, versionRegex, rawFrontmatter, "Wrong version")

	sResp, err := serializeWithoutOutputs(client, dResp.Notebook)
	assert.NoError(t, err)

	content := string(sResp.Result)

	assert.NotContains(t, content, "runme:\n")
	assert.NotContains(t, content, "id: \n")
	assert.NotContains(t, content, "version: v")
	assert.Contains(t, content, "invalid frontmatter")
	assert.Regexp(t, "^\\+\\+\\+\n", content)
	assert.NotRegexp(t, "^\n\n", content)

	assert.Contains(t, content, "```sh { name=foo id=123 }\n")
	assert.Contains(t, content, "```sh {\"id\":\""+testMockID+"\",\"name\":\"bar\"}\n")
	assert.Contains(t, content, "```js {\"id\":\""+testMockID+"\"}\n")
}

func Test_parserServiceServer_SessionFrontmatter(t *testing.T) {
	t.Run("Incomplete", func(t *testing.T) {
		doc := `---
runme:
  id: 01HJS33TJYXZ6KJPG2SZZ6D1H5
  version: v3
  session:
    id: 01HJS35FZ2K0JBWPVAXPMMVTGN
    updated: 2023-12-28 15:16:06-05:00
---

# Session but no document

`
		identity := parserv1.RunmeIdentity_RUNME_IDENTITY_UNSPECIFIED

		dResp, err := deserialize(client, doc, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]

		assert.True(t, ok)
		assert.Len(t, dResp.Notebook.Metadata, 4)
		assert.Contains(t, rawFrontmatter, "id: ")
		assert.Regexp(t, versionRegex, rawFrontmatter, "Valid version")

		sResp, err := serializeWithoutOutputs(client, dResp.Notebook)
		assert.NoError(t, err)

		content := string(sResp.Result)

		assert.Equal(t, doc, content)
	})
}

func Test_parserServiceServer_Ast(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		os.Setenv("RUNME_AST_METADATA", "true")

		identity := parserv1.RunmeIdentity_RUNME_IDENTITY_UNSPECIFIED

		content := "# Title\n\nSome content with [Link1](https://example1.com \"Link title 1\") [Link2](https://example2.com \"Link title2\")"

		resp, err := deserialize(client, content, identity)

		cells := resp.Notebook.Cells
		assert.NoError(t, err)
		assert.Len(t, cells, 2)

		assert.True(
			t,
			proto.Equal(
				&parserv1.Cell{
					Kind:  parserv1.CellKind_CELL_KIND_MARKUP,
					Value: "# Title",
					Metadata: map[string]string{
						"runme.dev/ast": `{"Children":[{"Kind":"Text","Text":"Title"}],"Kind":"Heading","Level":1,"RawText":"Title"}`,
					},
				},
				cells[0],
			),
		)
		assert.True(
			t,
			proto.Equal(
				&parserv1.Cell{
					Kind:  parserv1.CellKind_CELL_KIND_MARKUP,
					Value: "Some content with [Link1](https://example1.com \"Link title 1\") [Link2](https://example2.com \"Link title2\")",
					Metadata: map[string]string{
						"runme.dev/ast": `{"Children":[{"Kind":"Text","Text":"Some content with "},{"Children":[{"Kind":"Text","Text":"Link1"}],"Destination":"https://example1.com","Kind":"Link","Title":"Link title 1"},{"Kind":"Text","Text":" "},{"Children":[{"Kind":"Text","Text":"Link2"}],"Destination":"https://example2.com","Kind":"Link","Title":"Link title2"}],"Kind":"Paragraph","RawText":"Some content with [Link1](https://example1.com \"Link title 1\") [Link2](https://example2.com \"Link title2\")"}`,
					},
				},
				cells[1],
			),
		)
	})
}

func Test_parserServiceServer_Outputs(t *testing.T) {
	t.Run("Text", func(t *testing.T) {
		item := &parserv1.CellOutputItem{
			Data: []byte("\x1b[34mDoes it work?\r\n\x1b[32mYes, success!\x1b[1B\x1b[13D\x1b[0m"),
			Mime: "application/vnd.code.notebook.stdout",
			Type: "Buffer",
		}
		// todo(sebastian): until we have deserialization
		cell := &parserv1.Cell{
			Value:      "$ printf \"\\u001b[34mDoes it work?\\n\"\n$ sleep 2\n$ printf \"\\u001b[32mYes, success!\\x1b[0m\\n\"\n$ exit 16",
			Kind:       parserv1.CellKind_CELL_KIND_CODE,
			LanguageId: "sh",
			Outputs: []*parserv1.CellOutput{{
				Items: []*parserv1.CellOutputItem{item},
				ProcessInfo: &parserv1.CellOutputProcessInfo{
					ExitReason: &parserv1.ProcessInfoExitReason{
						Type: "exit",
						Code: &wrapperspb.UInt32Value{Value: 16},
					},
				},
			}},
			Metadata: map[string]string{"background": "false", "id": "01HF7B0KJPF469EG9ZVX256S75", "interactive": "true"},
			ExecutionSummary: &parserv1.CellExecutionSummary{
				Success: &wrapperspb.BoolValue{Value: true},
				Timing: &parserv1.ExecutionSummaryTiming{
					StartTime: &wrapperspb.Int64Value{Value: 1701280699458},
					EndTime:   &wrapperspb.Int64Value{Value: 1701280701754},
				},
			},
		}

		runmeVersionFm := "---\nrunme:\n  id: 01HF7B0KK32HBQ9X4AC2GPMZG5\n  version: %s"
		parsedFm := fmt.Sprintf(runmeVersionFm, "v2.0") + "\nsidebar_position: 1\ntitle: Examples\n---"
		notebook := &parserv1.Notebook{Cells: []*parserv1.Cell{cell}, Metadata: map[string]string{"runme.dev/frontmatter": parsedFm}}

		serializeOptions := &parserv1.SerializeRequestOptions{
			Outputs: &parserv1.SerializeRequestOutputOptions{Enabled: true, Summary: true},
			Session: &parserv1.RunmeSession{
				Id:       "01HJP23P1R57BPGEA17QDJXJE",
				Document: &parserv1.RunmeSessionDocument{RelativePath: "README.md"},
			},
		}
		resp, err := serializeWithOutputs(client, notebook, serializeOptions)
		assert.NoError(t, err)

		var content []string
		lines := strings.Split(string(resp.Result), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "    updated:") {
				content = append(content, "    updated: 0000-00-00 00:00:00Z")
				continue
			}
			content = append(content, line)
		}

		currentVersion := version.BaseVersion()
		expected := fmt.Sprintf(runmeVersionFm, currentVersion) + "\n  document:\n    relativePath: README.md\n  session:\n    id: 01HJP23P1R57BPGEA17QDJXJE\n    updated: 0000-00-00 00:00:00Z\nsidebar_position: 1\ntitle: Examples\n---\n\n```sh {\"background\":\"false\",\"id\":\"01HF7B0KJPF469EG9ZVX256S75\",\"interactive\":\"true\"}\n$ printf \"\\u001b[34mDoes it work?\\n\"\n$ sleep 2\n$ printf \"\\u001b[32mYes, success!\\x1b[0m\\n\"\n$ exit 16\n\n# Ran on 2023-11-29 17:58:19Z for 2.296s exited with 16\nDoes it work?\r\nYes, success!\n```\n"
		actual := strings.Join(content, "\n")

		assert.Equal(t, expected, actual)
	})
}

func deserialize(client parserv1.ParserServiceClient, content string, idt parserv1.RunmeIdentity) (*parserv1.DeserializeResponse, error) {
	return client.Deserialize(
		context.Background(),
		&parserv1.DeserializeRequest{
			Source: []byte(content),
			Options: &parserv1.DeserializeRequestOptions{
				Identity: idt,
			},
		},
	)
}

func serializeWithoutOutputs(client parserv1.ParserServiceClient, notebook *parserv1.Notebook) (*parserv1.SerializeResponse, error) {
	return serializeWithOutputs(client, notebook, &parserv1.SerializeRequestOptions{})
}

func serializeWithOutputs(client parserv1.ParserServiceClient, notebook *parserv1.Notebook, options *parserv1.SerializeRequestOptions) (*parserv1.SerializeResponse, error) {
	return client.Serialize(
		context.Background(),
		&parserv1.SerializeRequest{
			Notebook: notebook,
			Options:  options,
		},
	)
}
