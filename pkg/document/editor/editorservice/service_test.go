package editorservice

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stateful/runme/v3/internal/ulid"
	"github.com/stateful/runme/v3/internal/version"
	parserv1 "github.com/stateful/runme/v3/pkg/api/gen/proto/go/runme/parser/v1"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
		"```sh { name=bar }",
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

	conn, err := grpc.Dial(
		"",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(err)
	}

	client = parserv1.NewParserServiceClient(conn)
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
		assert.Contains(t, dResp.Notebook.Metadata, "runme.dev/id")

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]
		if tt.hasExtraFrontmatter {
			assert.True(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 3)
			assert.Contains(t, rawFrontmatter, "prop: value\n")
			assert.Contains(t, rawFrontmatter, "id: \"123\"\n")
			assert.Contains(t, rawFrontmatter, "version: v")
		} else {
			assert.False(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 2)
		}

		sResp, err := serializeWithIdentityPersistence(client, dResp.Notebook, identity)
		assert.NoError(t, err)
		content := string(sResp.Result)

		if tt.hasExtraFrontmatter {
			assert.Regexp(t, "^---\n", content)
		} else {
			assert.NotRegexp(t, "^---\n", content)
			assert.NotRegexp(t, "^\n\n", content)
		}

		assert.Contains(t, content, "```sh {\"id\":\"123\",\"name\":\"foo\"}\n")
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

		assert.Len(t, dResp.Notebook.Metadata, 3)

		if tt.hasExtraFrontmatter {
			assert.Contains(t, rawFrontmatter, "prop: value")
		}

		assert.Contains(t, rawFrontmatter, "id: "+testMockID)
		assert.Contains(t, rawFrontmatter, "version: "+version.BaseVersion())

		sResp, err := serializeWithIdentityPersistence(client, dResp.Notebook, identity)
		assert.NoError(t, err)

		content := string(sResp.Result)
		assert.Regexp(t, "^---\n", content)
		assert.Contains(t, content, "runme:\n")
		assert.Contains(t, content, "id: "+testMockID)
		assert.Contains(t, content, "version: "+version.BaseVersion())
		assert.Contains(t, content, "```sh {\"id\":\""+testMockID+"\",\"name\":\"foo\"}\n")
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

		assert.Len(t, dResp.Notebook.Metadata, 3)

		if tt.hasExtraFrontmatter {
			assert.Contains(t, rawFrontmatter, "prop: value")
		}

		assert.Contains(t, rawFrontmatter, "id: "+testMockID)
		assert.Regexpf(t, versionRegex, rawFrontmatter, "Wrong version")

		sResp, err := serializeWithIdentityPersistence(client, dResp.Notebook, identity)
		assert.NoError(t, err)

		content := string(sResp.Result)
		assert.Regexp(t, "^---\n", content)
		assert.Contains(t, content, "runme:\n")
		assert.Contains(t, content, "id: "+testMockID+"\n")
		assert.Contains(t, content, "version: "+version.BaseVersion()+"\n")
		assert.Contains(t, content, "```sh {\"id\":\"123\",\"name\":\"foo\"}\n")
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
		assert.Contains(t, dResp.Notebook.Metadata, "runme.dev/id")

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]

		if tt.hasExtraFrontmatter {
			assert.True(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 3)
			assert.Contains(t, rawFrontmatter, "prop: value\n")
			assert.Contains(t, rawFrontmatter, "id: \"123\"\n")
			assert.Regexp(t, versionRegex, rawFrontmatter, "Wrong version")
		} else {
			assert.False(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 2)
		}

		sResp, err := serializeWithIdentityPersistence(client, dResp.Notebook, identity)
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

		assert.Contains(t, content, "```sh {\"id\":\""+testMockID+"\",\"name\":\"foo\"}\n")
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
	assert.Len(t, dResp.Notebook.Metadata, 2)
	assert.Contains(t, rawFrontmatter, "prop: value\n")
	assert.NotContains(t, rawFrontmatter, "id: \"123\"\n")
	assert.NotRegexp(t, versionRegex, rawFrontmatter, "Wrong version")

	sResp, err := serializeWithIdentityPersistence(client, dResp.Notebook, identity)
	assert.NoError(t, err)

	content := string(sResp.Result)

	assert.NotContains(t, content, "runme:\n")
	assert.NotContains(t, content, "id: \"123\"\n")
	assert.NotContains(t, content, "version: v")
	assert.Contains(t, content, "prop: value\n")
	assert.Regexp(t, "^---\n", content)
	assert.NotRegexp(t, "^\n\n", content)

	assert.Contains(t, content, "```sh {\"id\":\""+testMockID+"\",\"name\":\"foo\"}\n")
	assert.Contains(t, content, "```sh {\"id\":\""+testMockID+"\",\"name\":\"bar\"}\n")
	assert.Contains(t, content, "```js {\"id\":\""+testMockID+"\"}\n")
}

func Test_NewFile_EmptyDocument_WithIdentityAll(t *testing.T) {
	doc := ""

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_ALL

	dResp, err := deserialize(client, doc, identity)
	assert.NoError(t, err)

	rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]
	assert.True(t, ok)
	assert.Len(t, dResp.Notebook.Metadata, 3)
	assert.Regexp(t, versionRegex, rawFrontmatter, "Wrong version")

	assert.NotNil(t, dResp.Notebook.Frontmatter)
	prasedRunmeID := dResp.Notebook.Frontmatter.Runme.Id
	assert.Contains(t, rawFrontmatter, "id: "+prasedRunmeID+"\n")

	sResp, err := serializeWithIdentityPersistence(client, dResp.Notebook, identity)
	assert.NoError(t, err)

	content := string(sResp.Result)

	assert.Contains(t, content, "runme:\n")
	assert.Contains(t, content, "id: "+prasedRunmeID+"\n")
	assert.Contains(t, content, "version: v")
	assert.Regexp(t, "^---\n", content)
}

func Test_EphemeralIdentity(t *testing.T) {
	doc := strings.Join([]string{
		"# Test identity integration with extension",
		"```sh\ngh auth --help\n```",
	}, "\n")

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_UNSPECIFIED

	dResp, err := deserialize(client, doc, identity)
	assert.NoError(t, err)

	assert.Len(t, dResp.Notebook.Metadata, 2)
	assert.Len(t, dResp.Notebook.Cells, 2)
	assert.NotContains(t, dResp.Notebook.Cells[1].Metadata, "id")
	assert.Contains(t, dResp.Notebook.Cells[1].Metadata, "runme.dev/id")

	sResp, err := serializeWithIdentityPersistence(client, dResp.Notebook, identity)
	assert.NoError(t, err)

	content := string(sResp.Result)

	assert.NotContains(t, content, "{\"id\":\"")
	assert.NotContains(t, dResp.Notebook.Metadata, "id")
	assert.NotContains(t, content, "runme:\n")
	assert.NotContains(t, content, "id: ")
	assert.NotContains(t, content, "version: v")
	assert.NotRegexp(t, "^---\n", content)
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

func serializeWithIdentityPersistence(client parserv1.ParserServiceClient, notebook *parserv1.Notebook, idt parserv1.RunmeIdentity) (*parserv1.SerializeResponse, error) {
	if idt == parserv1.RunmeIdentity_RUNME_IDENTITY_ALL || idt == parserv1.RunmeIdentity_RUNME_IDENTITY_CELL {
		persistIdentityLikeExtension(notebook)
	}
	return client.Serialize(
		context.Background(),
		&parserv1.SerializeRequest{
			Notebook: notebook,
		},
	)
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

// mimics what would happen on the extension side
func persistIdentityLikeExtension(notebook *parserv1.Notebook) {
	for _, cell := range notebook.Cells {
		// todo(sebastian): preserve original id when they are set?
		// if _, ok := cell.Metadata["id"]; ok {
		// 	break
		// }
		if v, ok := cell.Metadata["runme.dev/id"]; ok {
			cell.Metadata["id"] = v
		}
	}
}
