package codec

import (
	"errors"
	"net"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// Encode encodes the given data into a DNS message with a TXT record. The data is split into chunks of up to 255 bytes,
// as per the DNS TXT record specification. If the data exceeds the maximum allowed size for a DNS message, an error is
// returned. The function returns the encoded DNS message or an error if encoding fails.
func Encode(data []byte) (message []byte, err error) {
	const maxPayload = 65256 // Leaves room for the DNS header, TXT header, and string lengths.
	if len(data) > maxPayload {
		err = errors.New("data exceeds maximum DNS message size")
		return
	}

	var (
		chunks  int      = max(1, (len(data)+254)/255)
		txt     []string = make([]string, chunks)
		payload string   = string(data)
	)

	for i := range txt {
		var n int = min(len(payload), 255)
		txt[i] = payload[:n]
		payload = payload[n:]
	}

	var b dnsmessage.Builder = dnsmessage.NewBuilder(make([]byte, 0, 23+len(data)+chunks), dnsmessage.Header{})
	if err = b.StartAdditionals(); err != nil {
		return
	}

	if err = b.TXTResource(dnsmessage.ResourceHeader{
		Name:  dnsmessage.MustNewName("."),
		Class: dnsmessage.ClassINET,
	}, dnsmessage.TXTResource{
		TXT: txt,
	}); err != nil {
		return
	}

	message, err = b.Finish()
	return
}

// EncodeARecord encodes a DNS A record query for the given domain name. It constructs a DNS message with the specified
// domain name and returns the encoded message or an error if encoding fails. The function checks for valid domain name
// length and format before proceeding with the encoding.
func EncodeARecord(name string) (message []byte, err error) {
	if name == "" || len(name) > 253 {
		err = errors.New("invalid domain name")
		return
	}

	var b dnsmessage.Builder = dnsmessage.NewBuilder(make([]byte, 0, 23+len(name)), dnsmessage.Header{
		RecursionDesired: true,
	})

	if err = b.StartQuestions(); err != nil {
		return
	}

	if err = b.Question(dnsmessage.Question{
		Name:  dnsmessage.MustNewName(name + "."),
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
	}); err != nil {
		return
	}

	message, err = b.Finish()
	return
}

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
