//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/MohamedElashri/agrep/internal/playground"
)

var normalizeFunction js.Func

func main() {
	normalizeFunction = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 3 {
			return `{"error":"expected input, profile, and languages"}`
		}
		result, err := playground.Evaluate(args[0].String(), args[1].String(), args[2].String())
		if err != nil {
			data, _ := json.Marshal(map[string]string{"error": err.Error()})
			return string(data)
		}
		data, _ := json.Marshal(result)
		return string(data)
	})
	js.Global().Set("agrepNormalize", normalizeFunction)
	select {}
}
