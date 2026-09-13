package main

import (
	"fmt"
	"net"

	"github.com/z46-dev/dnscomms/codec"
	"golang.org/x/net/dns/dnsmessage"
)

func main() {
	var (
		message []byte
		resp    []byte
		err     error
	)

	if message, err = codec.EncodeARecord("google.com"); err != nil {
		panic(err)
	}

	if resp, err = codec.SendMessageToDNSServerAndGetResponse(message, "8.8.8.8:53"); err != nil {
		panic(err)
	}

	var response dnsmessage.Message
	if err := response.Unpack(resp); err != nil {
		panic(fmt.Errorf("decode DNS response: %w", err))
	}
	if !response.Header.Response {
		panic("received a DNS query instead of a response")
	}
	if response.Header.Truncated {
		panic("DNS response was truncated")
	}
	if response.Header.RCode != dnsmessage.RCodeSuccess {
		panic(fmt.Errorf("DNS server returned %v", response.Header.RCode))
	}

	found := false
	for _, answer := range response.Answers {
		if record, ok := answer.Body.(*dnsmessage.AResource); ok {
			fmt.Printf("Response from DNS server: %s -> %s\n", answer.Header.Name.String(), net.IP(record.A[:]))
			found = true
		}
	}
	if !found {
		fmt.Println("DNS response contained no A records")
	}
}
