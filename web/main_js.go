package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"syscall/js"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer"
	"github.com/stateful/runme/internal/renderer/md"
)

// These are variables so that they can be set during the build time.
var (
	BuildDate    = "unknown"
	BuildVersion = "0.0.0"
	Commit       = "unknown"
)

func main() {
	runme := map[string]interface{}{
		"deserialize": js.FuncOf(deserialize),
		"serialize":   js.FuncOf(serialize),
	}
	js.Global().Set("Runme", js.ValueOf(runme))

	select {}
}

var (
	parsed *document.ParsedSource
)

func assertParsed() js.Value {
	if parsed == nil {
		return toJSError(errors.New("call initialize() first"))
	}
	return js.Null()
}

func deserialize(this js.Value, args []js.Value) any {
	source := args[0].String()

	parsed = document.NewSource([]byte(source)).Parse()

	var buf bytes.Buffer
	if err := renderer.ToNotebookData(parsed, &buf); err != nil {
		return toJSError(err)
	}
	return buf.String()
}

func serialize(this js.Value, args []js.Value) any {
	source := args[0].String()

	var notebook document.Notebook
	if err := json.Unmarshal([]byte(source), &notebook); err != nil {
		return toJSError(err)
	}

	md.Render(parsed.Root(), []byte(source))

	return nil
}

func toJSError(err error) js.Value {
	if err == nil {
		return js.Null()
	}
	return js.Global().Get("Error").New(err.Error())
}
