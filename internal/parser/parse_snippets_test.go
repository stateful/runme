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
}
