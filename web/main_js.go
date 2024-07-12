package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/stateful/runme/v3/pkg/document/editor"
	"github.com/stateful/runme/v3/pkg/document/identity"
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

	handler := js.FuncOf(func(this js.Value, args []js.Value) any {
		resolve := args[0]
		reject := args[1]

		go func() {
			identity := identity.NewResolver(identity.DefaultLifecycleIdentity)
			notebook, err := editor.Deserialize([]byte(source), editor.Options{IdentityResolver: identity})
			if err != nil {
				reject.Invoke(toJSError(err))
				return
			}
			result, err := toMap(notebook)
			if err != nil {
				reject.Invoke(toJSError(err))
				return
			}
			resolve.Invoke(js.ValueOf(result))
		}()

		return nil
	})

	return js.Global().Get("Promise").New(handler)
}

func serialize(this js.Value, args []js.Value) any {
	// Notebook is sent as a JSON string. This is for convenience
	// to avoid a cumbersome conversion of a JS object
	// into a Go object would be needed.
	data := args[0].String()

	handler := js.FuncOf(func(this js.Value, args []js.Value) any {
		resolve := args[0]
		reject := args[1]

		go func() {
			var notebook editor.Notebook
			if err := json.Unmarshal([]byte(data), &notebook); err != nil {
				reject.Invoke(toJSError(err))
				return
			}
			result, err := editor.Serialize(&notebook, nil, editor.Options{})
			if err != nil {
				reject.Invoke(toJSError(err))
				return
			}
			resolve.Invoke(js.ValueOf(string(result)))
		}()

		return nil
	})

	return js.Global().Get("Promise").New(handler)
}

func toJSError(err error) js.Value {
	if err == nil {
		return js.Null()
	}
	return js.Global().Get("Error").New(err.Error())
}
