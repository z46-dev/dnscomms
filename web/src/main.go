//go:build js && wasm

package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"syscall/js"

	"github.com/z46-dev/dnscomms/codec"
	"golang.org/x/net/dns/dnsmessage"
)

type (
	simulationInput struct {
		Message    string `json:"message"`
		Domain     string `json:"domain"`
		RecordType string `json:"recordType"`
		PartSize   int    `json:"partSize"`
	}

	simulationPacket struct {
		Bytes int    `json:"bytes"`
		Hex   string `json:"hex"`
	}

	simulationResult struct {
		Packets    []simulationPacket `json:"packets"`
		Message    string             `json:"message"`
		InputBytes int                `json:"inputBytes"`
		WireBytes  int                `json:"wireBytes"`
		Error      string             `json:"error,omitempty"`
	}
)

// simulate runs the existing codec through a complete in-memory DNS round trip.
func simulate(input simulationInput) (result simulationResult, err error) {
	var (
		frames     [][]byte
		parts      []codec.ExfilPart
		decoded    []byte
		recordType dnsmessage.Type
	)
	switch input.RecordType {
	case "TXT":
		recordType = dnsmessage.TypeTXT
	case "A":
		recordType = dnsmessage.TypeA
	case "AAAA":
		recordType = dnsmessage.TypeAAAA
	default:
		err = errors.New("choose TXT, A, or AAAA")
		return
	}

	if len(input.Message) > 16384 || input.PartSize < 128 || input.PartSize > 1024 {
		err = errors.New("use a message up to 16384 bytes and a frame size from 128 to 1024")
		return
	}

	if frames, err = codec.EncodeExfil([]byte(input.Message), codec.ExfilOptions{PartSize: input.PartSize}); err != nil {
		return
	}

	for index, frame := range frames {
		var (
			packet    []byte
			recovered []byte
			part      codec.ExfilPart
		)
		if packet, err = codec.EncodeExfilDNS(frame, input.Domain, recordType, uint16(index)); err != nil {
			return
		}

		if recovered, err = codec.DecodeExfilDNS(packet); err != nil {
			return
		}

		if part, err = codec.DecodeExfil(recovered, nil); err != nil {
			return
		}

		parts = append(parts, part)
		result.Packets = append(result.Packets, simulationPacket{Bytes: len(packet), Hex: hex.EncodeToString(packet)})
		result.WireBytes += len(packet)
	}

	if decoded, err = codec.JoinExfil(parts); err == nil {
		result.Message = string(decoded)
		result.InputBytes = len(input.Message)
	}

	return
}

// main exposes one JSON interface for the browser and keeps the Go runtime alive.
func main() {
	js.Global().Set("dnscommsSimulate", js.FuncOf(func(_ js.Value, args []js.Value) (value any) {
		var (
			input   simulationInput
			result  simulationResult
			encoded []byte
			err     error
		)
		if len(args) != 1 || args[0].Type() != js.TypeString {
			result.Error = "expected a JSON simulation request"
		} else if err = json.Unmarshal([]byte(args[0].String()), &input); err != nil {
			result.Error = err.Error()
		} else if result, err = simulate(input); err != nil {
			result = simulationResult{Error: err.Error()}
		}

		encoded, _ = json.Marshal(result)
		value = string(encoded)
		return
	}))
	select {}
}
