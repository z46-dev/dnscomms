//go:build !js && !wasm

package codec

import (
	"net"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// SendMessageToDNSServerAndGetResponse sends a DNS message to the specified DNS server and waits for a response. It handles
// the connection setup, message sending, and response reading. The function returns the decoded DNS response message or an
// error if any step in the process fails. It uses a timeout to avoid indefinite blocking during network operations.
func SendMessageToDNSServerAndGetResponse(message []byte, server string) (resp dnsmessage.Message, err error) {
	if _, _, err = net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}

	const timeout = 5 * time.Second
	var conn net.Conn

	if conn, err = net.DialTimeout("udp", server, timeout); err != nil {
		return
	}

	defer conn.Close()

	if err = conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return
	}

	if _, err = conn.Write(message); err != nil {
		return
	}

	var (
		response []byte = make([]byte, 65535)
		n        int
	)

	if n, err = conn.Read(response); err != nil {
		return
	}

	response = response[:n]
	err = resp.Unpack(response)
	return
}
