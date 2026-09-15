package codec

import (
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

type EncodeableType struct {
	RecordType       dnsmessage.Type
	MaxPayloadLength int
}

var EncodeableTypes = map[dnsmessage.Type]EncodeableType{
	dnsmessage.TypeA: {
		RecordType:       dnsmessage.TypeA,
		MaxPayloadLength: 187,
	},
	dnsmessage.TypeAAAA: {
		RecordType:       dnsmessage.TypeAAAA,
		MaxPayloadLength: 187,
	},
	dnsmessage.TypeTXT: {
		RecordType:       dnsmessage.TypeTXT,
		MaxPayloadLength: 65256,
	},
}

// Encodes a DNS query message, turning the data into base64 and encoding it in an appropriate DNS record type.
func Encode(recordType dnsmessage.Type, data []byte) (message []byte, err error) {
	var (
		encodeableType EncodeableType
		ok             bool
	)

	if encodeableType, ok = EncodeableTypes[recordType]; !ok {
		err = errors.New("unsupported record type")
		return
	}

	if len(data) > encodeableType.MaxPayloadLength {
		err = errors.New("data exceeds maximum payload length for the specified record type")
		return
	}

	var b dnsmessage.Builder = dnsmessage.NewBuilder(make([]byte, 0, 23+len(data)), dnsmessage.Header{
		RecursionDesired: true,
	})

	if err = b.StartQuestions(); err != nil {
		return
	}

	var name string = "."
	if encodeableType.RecordType == dnsmessage.TypeA || encodeableType.RecordType == dnsmessage.TypeAAAA {
		var (
			encoded string = base64.RawURLEncoding.EncodeToString(data)
			labels  []string
		)

		for len(encoded) > 0 {
			var length int = min(63, len(encoded))
			labels = append(labels, encoded[:length])
			encoded = encoded[length:]
		}

		name = strings.Join(labels, ".") + "."
	}

	if err = b.Question(dnsmessage.Question{
		Name:  dnsmessage.MustNewName(name),
		Type:  encodeableType.RecordType,
		Class: dnsmessage.ClassINET,
	}); err != nil {
		return
	}

	if encodeableType.RecordType == dnsmessage.TypeTXT {
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
	}

	message, err = b.Finish()
	return
}

// Decode extracts the payload from an encoded DNS query.
func Decode(message []byte) (recordType dnsmessage.Type, data []byte, err error) {
	var query dnsmessage.Message
	if err = query.Unpack(message); err != nil {
		return
	}

	if len(query.Questions) != 1 {
		err = errors.New("expected one DNS question")
		return
	}

	recordType = query.Questions[0].Type

	switch recordType {
	case dnsmessage.TypeA, dnsmessage.TypeAAAA:
		data, err = base64.RawURLEncoding.DecodeString(strings.ReplaceAll(strings.TrimSuffix(query.Questions[0].Name.String(), "."), ".", ""))
	case dnsmessage.TypeTXT:
		if len(query.Additionals) != 1 || query.Additionals[0].Header.Type != dnsmessage.TypeTXT {
			err = errors.New("expected one additional TXT resource")
			return
		}

		var (
			txtRecord *dnsmessage.TXTResource
			ok        bool
		)

		if txtRecord, ok = query.Additionals[0].Body.(*dnsmessage.TXTResource); !ok {
			err = errors.New("failed to decode TXT resource")
			return
		}

		for _, chunk := range txtRecord.TXT {
			data = append(data, chunk...)
		}
	default:
		err = errors.New("unsupported record type in DNS query")
	}

	return
}
