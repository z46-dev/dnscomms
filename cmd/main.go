package main

import (
	"fmt"
	"net"

	"github.com/z46-dev/dnscomms/codec"
	"golang.org/x/net/dns/dnsmessage"
)

func main() {
	var (
		message  []byte
		response dnsmessage.Message
		err      error
	)

	if message, err = codec.EncodeARecord("google.com"); err != nil {
		panic(err)
	}

	if response, err = codec.SendMessageToDNSServerAndGetResponse(message, "8.8.8.8:53"); err != nil {
		panic(err)
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

	var (
		found  bool = false
		record *dnsmessage.AResource
		ok     bool
	)

	for _, answer := range response.Answers {
		if record, ok = answer.Body.(*dnsmessage.AResource); ok {
			fmt.Printf("Response from DNS server: %s -> %s\n", answer.Header.Name.String(), net.IP(record.A[:]))
			found = true
		}
	}

	if !found {
		fmt.Println("DNS response contained no A records")
	}
}
