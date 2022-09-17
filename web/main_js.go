package main

import (
	"bytes"
	"encoding/json"
	"syscall/js"

	"github.com/stateful/rdme/internal/document"
)

// These are variables so that they can be set during the build time.
var (
	BuildDate    = "unknown"
	BuildVersion = "0.0.0"
	Commit       = "unknown"
)

func main() {
<<<<<<< HEAD
	js.Global().Set("GetSnippets", js.FuncOf(GetSnippets))
	js.Global().Set("GetDocument", js.FuncOf(GetDocument))
=======
	js.Global().Set("GetBlocks", js.FuncOf(GetBlocks))
>>>>>>> 0f59f74 (Refactor parser and rename it to document package)

	select {}
}

func GetBlocks(this js.Value, args []js.Value) interface{} {
	readme := args[0].String()

<<<<<<< HEAD
	p := parser.New([]byte(readme))
	snippets := p.Snippets()
	for _, s := range snippets {
		s.Lines = s.GetLines()
=======
	blocks := document.NewSource([]byte(readme)).Parse(nil).Blocks()

	var result []interface{}

	for _, block := range blocks {
		var lines []interface{}
		for _, line := range block.Lines() {
			lines = append(lines, line)
		}
		entry := map[string]interface{}{
			"name":        block.Name(),
			"description": block.Intro(),
			"content":     block.Content(),
			"executable":  block.Executable(),
			"lines":       lines,
		}
		result = append(result, entry)
>>>>>>> 0f59f74 (Refactor parser and rename it to document package)
	}
	b, _ := json.Marshal(snippets)

	var dynamic []interface{}
	json.Unmarshal(b, &dynamic)

	return dynamic
}

func GetDocument(this js.Value, args []js.Value) interface{} {
	readme := args[0].String()
	p := parser.New([]byte(readme))

	var b bytes.Buffer
	p.Render(&b)

	dynamic := make(map[string]interface{})
	json.Unmarshal(b.Bytes(), &dynamic)

	return dynamic
}
