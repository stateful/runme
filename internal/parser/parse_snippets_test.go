package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser_Snippets(t *testing.T) {
	p0 := New([]byte(`
Snippet without proper annotations:
` + "```" + `
go run main.go
` + "```"))
	snippets0 := p0.Snippets()
	assert.Len(t, snippets0, 1)
	assert.EqualValues(t, []string{"go-run"}, snippets0.Names())
	assert.EqualValues(t, []string{"go run main.go"}, snippets0[0].Cmds())
	assert.EqualValues(t, "go run main.go", snippets0[0].FirstCmd())

	p1 := New([]byte(`
Snippet without proper annotations:
` + "```sh {}" + `
go run main.go
` + "```"))
	snippets1 := p1.Snippets()
	assert.Len(t, snippets1, 1)
	assert.EqualValues(t, []string{"go-run"}, snippets1.Names())

	p2 := New([]byte(`
Snippet without proper annotations:
` + "```sh" + `
go run main.go
` + "```"))
	snippets2 := p2.Snippets()
	assert.Len(t, snippets2, 1)
	assert.EqualValues(t, []string{"go-run"}, snippets2.Names())

	p3 := New([]byte(`
Snippet without proper annotations:
` + "```sh {name=run}" + `
go run main.go
` + "```"))
	snippets3 := p3.Snippets()
	assert.Len(t, snippets3, 1)
	assert.EqualValues(t, []string{"run"}, snippets3.Names())

	p4 := New([]byte(`
Snippet without proper annotations:
` + "```sh { name=run }" + `
go run main.go
` + "```"))
	snippets4 := p4.Snippets()
	assert.Len(t, snippets4, 1)
	assert.EqualValues(t, []string{"run"}, snippets4.Names())

	p5 := New([]byte(`
Feedback can be patched:
` + "```sh { name=patch-feedback }" + `
$ curl -X PATCH -H "Content-Type: application/json" localhost:8080/feedback/a02b6b5f-46c4-40ff-8160-ff7d55b8ca6f/ -d '{"message": "Modified!"}'
{"id":"a02b6b5f-46c4-40ff-8160-ff7d55b8ca6f"}
` + "```"))
	snippets5 := p5.Snippets()
	assert.Len(t, snippets5, 1)
	assert.EqualValues(t, []string{"patch-feedback"}, snippets5.Names())
	p5Snippet, _ := snippets5.Lookup("patch-feedback")
	assert.Equal(t, []string{`curl -X PATCH -H "Content-Type: application/json" localhost:8080/feedback/a02b6b5f-46c4-40ff-8160-ff7d55b8ca6f/ -d '{"message": "Modified!"}'`}, p5Snippet.Cmds())
}
