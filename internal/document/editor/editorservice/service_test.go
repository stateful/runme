package editorservice

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stateful/runme/internal/document/editor"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/internal/identity"
	"github.com/stateful/runme/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

var testMockID = identity.GenerateID()

func TestMain(m *testing.M) {
	identity.MockGenerator(testMockID)

	code := m.Run()
	identity.ResetGenerator()
	os.Exit(code)
}

func Test_parserServiceServer(t *testing.T) {
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
	require.NoError(t, err)
	client := parserv1.NewParserServiceClient(conn)

	t.Run("Basic", func(t *testing.T) {
		os.Setenv("RUNME_AST_METADATA", "true")

		resp, err := client.Deserialize(
			context.Background(),
			&parserv1.DeserializeRequest{
				Source: []byte("# Title\n\nSome content with [Link1](https://example1.com \"Link title 1\") [Link2](https://example2.com \"Link title2\")"),
			},
		)

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

	t.Run("Frontmatter Identity RUNME_IDENTITY_ALL", func(t *testing.T) {
		frontMatter := fmt.Sprintf(`---
prop: value
runme:
  id: %s
  version: "%s"
---`, testMockID, version.BaseVersion())
		content := `# Hello

Some content
`
		dResp, err := client.Deserialize(
			context.Background(),
			&parserv1.DeserializeRequest{
				Source: []byte(frontMatter + "\n" + content),
			},
		)
		assert.NoError(t, err)
		assert.Len(t, dResp.Notebook.Cells, 2)
		assert.Equal(
			t,
			frontMatter,
			dResp.Notebook.Metadata[editor.PrefixAttributeName(editor.InternalAttributePrefix, editor.FrontmatterKey)],
		)

		sResp, err := client.Serialize(
			context.Background(),
			&parserv1.SerializeRequest{
				Notebook: dResp.Notebook,
				Identity: parserv1.RunmeIdentity_RUNME_IDENTITY_ALL,
			},
		)
		expected := frontMatter + "\n\n" + content
		actual := string(sResp.Result)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("Frontmatter Identity RUNME_IDENTITY_UNSPECIFIED", func(t *testing.T) {
		frontMatter := `---
prop: value
---`
		content := `# Hello

Some content
`
		dResp, err := client.Deserialize(
			context.Background(),
			&parserv1.DeserializeRequest{
				Source: []byte(frontMatter + "\n" + content),
			},
		)
		assert.NoError(t, err)
		assert.Len(t, dResp.Notebook.Cells, 2)
		sResp, err := client.Serialize(
			context.Background(),
			&parserv1.SerializeRequest{
				Notebook: dResp.Notebook,
				Identity: parserv1.RunmeIdentity_RUNME_IDENTITY_UNSPECIFIED,
			},
		)

		expected := frontMatter + "\n\n" + content
		actual := string(sResp.Result)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("Frontmatter Identity RUNME_IDENTITY_CELL", func(t *testing.T) {
		frontMatter := strings.Join([]string{
			"---",
			"prop: value",
			"---",
		}, "\n")

		content := strings.Join([]string{
			"# Hello",
			"",
			"Some content",
			"",
			"```sh { name=foo }",
			`echo "Hello"`,
			"```",
		}, "\n")

		expectedContent := strings.Join([]string{
			"# Hello",
			"",
			"Some content",
			"",
			fmt.Sprintf("```sh { name=foo id=%s }", testMockID),
			`echo "Hello"`,
			"```",
		}, "\n")

		dResp, err := client.Deserialize(
			context.Background(),
			&parserv1.DeserializeRequest{
				Source: []byte(frontMatter + "\n" + content),
			},
		)

		assert.NoError(t, err)
		assert.Len(t, dResp.Notebook.Cells, 3)
		sResp, err := client.Serialize(
			context.Background(),
			&parserv1.SerializeRequest{
				Notebook: dResp.Notebook,
				Identity: parserv1.RunmeIdentity_RUNME_IDENTITY_CELL,
			},
		)

		expected := frontMatter + "\n\n" + expectedContent
		actual := string(sResp.Result)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})
}
