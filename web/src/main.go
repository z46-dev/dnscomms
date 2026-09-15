//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/z46-dev/dnscomms/serverless"
)

var network *serverless.Network = serverless.New(serverless.CloudflareDNS)

// main exposes a JSON command interface and retains the browser runtime.
func main() {
	js.Global().Set("dnscommsSimulate", js.FuncOf(func(_ js.Value, args []js.Value) (value any) {
		var (
			input   serverless.Input
			encoded []byte
			err     error
		)
		if len(args) != 1 || args[0].Type() != js.TypeString {
			value = `{"error":"expected a JSON command"}`
			return
		}
		if err = json.Unmarshal([]byte(args[0].String()), &input); err == nil {
			err = network.Apply(input)
		}
		if err != nil {
			encoded, _ = json.Marshal(map[string]string{"error": err.Error()})
		} else {
			encoded, _ = json.Marshal(network)
		}
		value = string(encoded)
		return
	}))
	js.Global().Set("dnscommsCanvas", js.FuncOf(func(_ js.Value, args []js.Value) (value any) {
		if len(args) == 1 {
			value = attachCanvas(args[0].Bool())
		}
		return
	}))
	select {}
}
