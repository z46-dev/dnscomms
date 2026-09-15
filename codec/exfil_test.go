package codec_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/z46-dev/dnscomms/codec"
	"golang.org/x/net/dns/dnsmessage"
)

func TestDNSPayloadRoundTrip(t *testing.T) {
	for _, recordType := range []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA, dnsmessage.TypeTXT} {
		var payload []byte = bytes.Repeat([]byte{0x00, 0xff, 0x73}, codec.EncodeableTypes[recordType].MaxPayloadLength/3)
		payload = append(payload, bytes.Repeat([]byte{0x42}, codec.EncodeableTypes[recordType].MaxPayloadLength-len(payload))...)

		var (
			encoded, decoded []byte
			decodedType      dnsmessage.Type
			err              error
		)

		if encoded, err = codec.Encode(recordType, payload); err != nil {
			t.Fatalf("encode %v at maximum size: %v", recordType, err)
		}

		if decodedType, decoded, err = codec.Decode(encoded); err != nil {
			t.Fatalf("decode %v at maximum size: %v", recordType, err)
		}

		assert.Equal(t, recordType, decodedType)
		assert.Equal(t, payload, decoded)

		if _, err = codec.Encode(recordType, append(payload, 0)); err == nil {
			t.Fatalf("expected %v to reject an oversized payload", recordType)
		}
	}
}

func TestExfilLoop(t *testing.T) {
	var (
		message, decodedMessage []byte = []byte("Supercalifragilisticexpialidocious"), []byte{}
		messages                []codec.ExfilRequest
		err                     error
	)

	if messages, err = codec.EncodeExfilRequest(message, codec.ExfilOptions{
		Servers:  []string{"example.com"},
		Duration: 10,
	}); err != nil {
		t.Fatalf("failed to encode exfil request: %v", err)
	}

	if decodedMessage, err = codec.RebuildExfilData(messages); err != nil {
		t.Fatalf("failed to decode exfil request: %v", err)
	}

	assert.Equal(t, message, decodedMessage, "decoded message does not match original")
}

func TestExfilEncodingLoop(t *testing.T) {
	var (
		message, decodedMessage   []byte = []byte("Supercalifragilisticexpialidocious"), []byte{}
		messages, decodedMessages []codec.ExfilRequest
		messageBodies             [][]byte
		err                       error
	)

	if messages, err = codec.EncodeExfilRequest(message, codec.ExfilOptions{
		Servers:  []string{"example.com"},
		Duration: 10,
	}); err != nil {
		t.Fatalf("failed to encode exfil request: %v", err)
	}

	for _, msg := range messages {
		var encoded []byte
		if encoded, err = msg.Encode(); err != nil {
			t.Fatalf("failed to encode exfil request: %v", err)
		}

		messageBodies = append(messageBodies, encoded)
	}

	for _, body := range messageBodies {
		var decoded codec.ExfilRequest
		if decoded, err = codec.DecodeExfilRequest(body); err != nil {
			t.Fatalf("failed to decode exfil request: %v", err)
		}

		decodedMessages = append(decodedMessages, decoded)
	}

	if decodedMessage, err = codec.RebuildExfilData(decodedMessages); err != nil {
		t.Fatalf("failed to rebuild exfil data: %v", err)
	}

	assert.Equal(t, message, decodedMessage, "decoded message does not match original")
}
