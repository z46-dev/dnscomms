//go:build js && wasm

package codec

import (
	"errors"

	"golang.org/x/net/dns/dnsmessage"
)

// SendMessageToDNSServerAndGetResponse is not implemented in this build.
func SendMessageToDNSServerAndGetResponse(message []byte, server string) (resp dnsmessage.Message, err error) {
	err = errors.New("SendMessageToDNSServerAndGetResponse is not implemented in this build")
	return
}
