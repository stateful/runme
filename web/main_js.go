package main

import (
	"encoding/json"
	"errors"
	"syscall/js"

	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/document/edit"
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

var editor = edit.New()

func assertEditor() js.Value {
	if editor == nil {
		return toJSError(errors.New("call deserialize() first"))
	}
	return js.Null()
}

func toMap(o any) (map[string]any, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func deserialize(this js.Value, args []js.Value) any {
	source := args[0].String()
	cells, err := editor.Deserialize([]byte(source))
	if err != nil {
		return toJSError(err)
	}
	result, err := toMap(document.Notebook{Cells: cells})
	if err != nil {
		return toJSError(err)
	}
	return result
}

func serialize(this js.Value, args []js.Value) any {
	data, err := json.Marshal(args[0])
	if err != nil {
		return toJSError(err)
	}
	var notebook document.Notebook
	if err := json.Unmarshal(data, &notebook); err != nil {
		return toJSError(err)
	}
	data, err = editor.Serialize(notebook.Cells)
	if err != nil {
		return toJSError(err)
	}
	return string(data)
}

func toJSError(err error) js.Value {
	if err == nil {
		return js.Null()
	}
	return js.Global().Get("Error").New(err.Error())
}
