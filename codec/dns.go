package codec

import (
	"encoding/base64"
	"errors"

	"golang.org/x/net/dns/dnsmessage"
)

type EncodeableType struct {
	RecordType       dnsmessage.Type
	MaxPayloadLength int
}

var EncodeableTypes = map[dnsmessage.Type]EncodeableType{
	dnsmessage.TypeA: {
		RecordType:       dnsmessage.TypeA,
		MaxPayloadLength: 253,
	},
	dnsmessage.TypeAAAA: {
		RecordType:       dnsmessage.TypeAAAA,
		MaxPayloadLength: 253,
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
		name = base64.RawURLEncoding.EncodeToString(data) + "."
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

	return
}

// Decode decodes a DNS response message, extracting
func Decode(message []byte) (recordType dnsmessage.Type, data []byte, err error) {
	var resp dnsmessage.Message
	if err = resp.Unpack(message); err != nil {
		return
	}

	if len(resp.Answers) == 0 {
		err = errors.New("no answers in DNS response")
		return
	}

	var (
		answer dnsmessage.Resource
		ok     bool
	)

	answer = resp.Answers[0]
	recordType = answer.Header.Type

	switch recordType {
	case dnsmessage.TypeA:
		var aRecord *dnsmessage.AResource
		if aRecord, ok = answer.Body.(*dnsmessage.AResource); ok {
			data = aRecord.A[:]
		} else {
			err = errors.New("failed to decode A record")
		}
	case dnsmessage.TypeAAAA:
		var aaaaRecord *dnsmessage.AAAAResource
		if aaaaRecord, ok = answer.Body.(*dnsmessage.AAAAResource); ok {
			data = aaaaRecord.AAAA[:]
		} else {
			err = errors.New("failed to decode AAAA record")
		}
	case dnsmessage.TypeTXT:
		var txtRecord *dnsmessage.TXTResource
		if txtRecord, ok = answer.Body.(*dnsmessage.TXTResource); ok {
			data = []byte{}
			for _, txt := range txtRecord.TXT {
				data = append(data, []byte(txt)...)
			}
		} else {
			err = errors.New("failed to decode TXT record")
		}
	default:
		err = errors.New("unsupported record type in DNS response")
	}

	return
}
