//go:build js && wasm

package main

import (
	"fmt"

	"github.com/z46-dev/dnscomms/codec"
)

func main() {
	var (
		msg []byte
		err error
	)

	if msg, err = codec.EncodeARecord("google.com"); err != nil {
		panic(err)
	}

	fmt.Printf("get google.com A: %x\n", msg)

	select {}
}
