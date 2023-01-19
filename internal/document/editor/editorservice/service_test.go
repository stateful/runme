package editorservice

import (
	"context"
	"net"
	"testing"

	"github.com/stateful/runme/internal/document/editor"
	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

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
		resp, err := client.Deserialize(
			context.Background(),
			&parserv1.DeserializeRequest{
				Source: []byte("# Title\n\nSome content"),
			},
		)
		assert.NoError(t, err)
		assert.Len(t, resp.Notebook.Cells, 2)
		assert.True(
			t,
			proto.Equal(
				&parserv1.Cell{
					Kind:  parserv1.CellKind_CELL_KIND_MARKUP,
					Value: "# Title",
				},
				resp.Notebook.Cells[0],
			),
		)
		assert.True(
			t,
			proto.Equal(
				&parserv1.Cell{
					Kind:  parserv1.CellKind_CELL_KIND_MARKUP,
					Value: "Some content",
				},
				resp.Notebook.Cells[1],
			),
		)
	})

	t.Run("Frontmatter", func(t *testing.T) {
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
		assert.Equal(
			t,
			frontMatter,
			dResp.Notebook.Metadata[editor.FrontmatterKey],
		)

		sResp, err := client.Serialize(
			context.Background(),
			&parserv1.SerializeRequest{
				Notebook: dResp.Notebook,
			},
		)
		assert.NoError(t, err)
		assert.Equal(t, frontMatter+"\n\n"+content, string(sResp.Result))
	})
}
