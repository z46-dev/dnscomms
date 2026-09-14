package codec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

const maxDNSMessageSize = 65535

// EncodeExfilDNS wraps one frame in a DNS response using TXT, A, or AAAA answers.
// Each record starts with a two-byte index; the combined data starts with a four-byte
// frame length. Address records are zero-padded. DNS IDs belong to the caller.
// Output is a DNS message, without IP/UDP headers or a TCP length prefix. Messages
// larger than 512 bytes require a transport supporting their size (e.g. DNS over TCP).
func EncodeExfilDNS(frame []byte, name string, recordType dnsmessage.Type, id uint16) (packet []byte, err error) {
	var (
		message dnsmessage.Message
		owner   dnsmessage.Name
		width   int
		data    []byte
	)

	if width, err = exfilDNSWidth(recordType); err != nil {
		return
	}

	if len(frame) < ExfilHeaderSize || len(frame) > maxDNSMessageSize {
		err = errors.New("invalid DNS frame size")
		return
	}

	if name == "" {
		err = errors.New("DNS owner name is required")
		return
	}

	if owner, err = dnsmessage.NewName(strings.TrimSuffix(name, ".") + "."); err != nil {
		return
	}

	message.Header = dnsmessage.Header{ID: id, Response: true}
	message.Questions = []dnsmessage.Question{{Name: owner, Type: recordType, Class: dnsmessage.ClassINET}}
	data = make([]byte, 4+len(frame))
	binary.BigEndian.PutUint32(data, uint32(len(frame)))
	copy(data[4:], frame)

	for offset := 0; offset < len(data); offset += width - 2 {
		var (
			body     []byte = make([]byte, width)
			resource dnsmessage.Resource
		)

		binary.BigEndian.PutUint16(body, uint16(offset/(width-2)))
		copy(body[2:], data[offset:min(offset+width-2, len(data))])
		resource.Header = dnsmessage.ResourceHeader{Name: owner, Class: dnsmessage.ClassINET}
		switch recordType {
		case dnsmessage.TypeTXT:
			// TXT strings need no padding and may contain arbitrary bytes.
			resource.Body = &dnsmessage.TXTResource{TXT: []string{string(body[:2+min(width-2, len(data)-offset)])}}
		case dnsmessage.TypeA:
			resource.Body = &dnsmessage.AResource{A: [4]byte(body)}
		case dnsmessage.TypeAAAA:
			resource.Body = &dnsmessage.AAAAResource{AAAA: [16]byte(body)}
		}

		message.Answers = append(message.Answers, resource)
	}

	if packet, err = message.Pack(); err != nil {
		return
	}

	if len(packet) > maxDNSMessageSize {
		packet = nil
		err = errors.New("frame exceeds DNS message capacity; use a smaller ExfilOptions.PartSize")
	}

	return
}

// DecodeExfilDNS recovers one frame from a complete response, regardless of record order.
// Pass the result to DecodeExfil for protocol validation and optional decryption.
func DecodeExfilDNS(packet []byte) (frame []byte, err error) {
	var (
		message dnsmessage.Message
		width   int
		chunks  map[uint16][]byte
		data    []byte
		size    uint32
	)

	if len(packet) > maxDNSMessageSize {
		err = errors.New("DNS message exceeds size limit")
		return
	}

	if err = message.Unpack(packet); err != nil {
		return
	}

	if !message.Response || message.Truncated || message.OpCode != 0 || message.RCode != dnsmessage.RCodeSuccess || len(message.Questions) != 1 || len(message.Answers) == 0 {
		err = errors.New("expected a complete successful DNS response with one question and answers")
		return
	}

	if message.Questions[0].Class != dnsmessage.ClassINET {
		err = errors.New("expected Internet-class DNS records")
		return
	}

	if width, err = exfilDNSWidth(message.Questions[0].Type); err != nil {
		return
	}

	chunks = make(map[uint16][]byte)
	for _, answer := range message.Answers {
		var (
			body     []byte
			index    uint16
			existing []byte
			ok       bool
		)

		if answer.Header.Type != message.Questions[0].Type || answer.Header.Class != dnsmessage.ClassINET || !strings.EqualFold(answer.Header.Name.String(), message.Questions[0].Name.String()) {
			err = errors.New("inconsistent DNS answer type, class, or owner")
			return
		}

		switch record := answer.Body.(type) {
		case *dnsmessage.TXTResource:
			body = []byte(strings.Join(record.TXT, ""))
		case *dnsmessage.AResource:
			body = record.A[:]
		case *dnsmessage.AAAAResource:
			body = record.AAAA[:]
		}

		if len(body) <= 2 || len(body) > width {
			err = errors.New("invalid DNS carrier record length")
			return
		}

		index = binary.BigEndian.Uint16(body)
		if existing, ok = chunks[index]; ok {
			if !bytes.Equal(existing, body[2:]) {
				err = errors.New("conflicting DNS carrier records")
				return
			}
		} else {
			chunks[index] = body[2:]
		}
	}

	for index := 0; index < len(chunks); index++ {
		var (
			chunk []byte
			ok    bool
		)

		if chunk, ok = chunks[uint16(index)]; !ok || (index < len(chunks)-1 && len(chunk) != width-2) {
			err = errors.New("missing or short DNS carrier record")
			return
		}

		data = append(data, chunk...)
	}

	if len(data) < 4 {
		err = errors.New("missing DNS carrier length")
		return
	}

	size = binary.BigEndian.Uint32(data)
	if size < ExfilHeaderSize || size > uint32(len(data)-4) || len(chunks) != (int(size)+4+width-3)/(width-2) {
		err = errors.New("invalid DNS carrier frame length")
		return
	}

	if message.Questions[0].Type == dnsmessage.TypeTXT && len(data) != int(size)+4 {
		err = errors.New("unexpected TXT carrier padding")
		return
	}

	for _, padding := range data[4+int(size):] {
		if padding != 0 {
			err = errors.New("invalid DNS carrier padding")
			return
		}
	}

	frame = data[4 : 4+int(size)]
	return
}

// exfilDNSWidth returns the carrier record size, including the ordering prefix.
func exfilDNSWidth(recordType dnsmessage.Type) (width int, err error) {
	switch recordType {
	case dnsmessage.TypeTXT:
		width = 255
	case dnsmessage.TypeA:
		width = 4
	case dnsmessage.TypeAAAA:
		width = 16
	default:
		err = errors.New("DNS carrier must be TXT, A, or AAAA")
	}

	return
}
