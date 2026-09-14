package codec

import (
	"errors"
	"net"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func Encode(data []byte) ([]byte, error) {
	const maxPayload = 65256 // Leaves room for the DNS header, TXT header, and string lengths.
	if len(data) > maxPayload {
		return nil, errors.New("data exceeds maximum DNS message size")
	}

	chunks := (len(data) + 254) / 255
	if chunks == 0 {
		chunks = 1
	}
	txt := make([]string, chunks)
	payload := string(data)
	for i := range txt {
		n := min(len(payload), 255)
		txt[i] = payload[:n]
		payload = payload[n:]
	}

	b := dnsmessage.NewBuilder(make([]byte, 0, 23+len(data)+chunks), dnsmessage.Header{})
	if err := b.StartAdditionals(); err != nil {
		return nil, err
	}
	if err := b.TXTResource(dnsmessage.ResourceHeader{
		Name:  dnsmessage.MustNewName("."),
		Class: dnsmessage.ClassINET,
	}, dnsmessage.TXTResource{TXT: txt}); err != nil {
		return nil, err
	}
	return b.Finish()
}

func EncodeARecord(name string) (message []byte, err error) {
	if name == "" || len(name) > 253 {
		err = errors.New("invalid domain name")
		return
	}

	var b dnsmessage.Builder = dnsmessage.NewBuilder(make([]byte, 0, 23+len(name)), dnsmessage.Header{RecursionDesired: true})
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
