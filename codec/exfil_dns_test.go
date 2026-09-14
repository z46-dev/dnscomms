package codec

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

// TestExfilDNSRoundTrip covers all carriers through real DNS wire packets and frame reassembly.
func TestExfilDNSRoundTrip(t *testing.T) {
	for _, recordType := range []dnsmessage.Type{dnsmessage.TypeTXT, dnsmessage.TypeA, dnsmessage.TypeAAAA} {
		for _, key := range [][]byte{nil, bytes.Repeat([]byte{1}, 32)} {
			for _, input := range [][]byte{nil, bytes.Repeat([]byte{0, 255, 42}, 1000)} {
				var (
					frames                    [][]byte
					parts                     []ExfilPart
					part                      ExfilPart
					packet, recovered, output []byte
					message                   dnsmessage.Message
					err                       error
				)

				if frames, err = EncodeExfil(input, ExfilOptions{PartSize: 700, Key: key}); err != nil {
					t.Fatal(err)
				}

				for _, frame := range frames {
					if packet, err = EncodeExfilDNS(frame, "demo.example.", recordType, 123); err != nil {
						t.Fatal(err)
					}

					if err = message.Unpack(packet); err != nil {
						t.Fatal(err)
					}

					if message.ID != 123 || !message.Response || message.Questions[0].Type != recordType {
						t.Fatal("incorrect DNS header or question")
					}

					for _, answer := range message.Answers {
						if answer.Header.Type != recordType {
							t.Fatal("incorrect DNS resource type")
						}
					}

					slices.Reverse(message.Answers)
					message.Answers = append(message.Answers, message.Answers[0])
					if packet, err = message.Pack(); err != nil {
						t.Fatal(err)
					}

					if recovered, err = DecodeExfilDNS(packet); err != nil || !bytes.Equal(recovered, frame) {
						t.Fatalf("%v carrier round trip failed: %v", recordType, err)
					}

					if part, err = DecodeExfil(recovered, key); err != nil {
						t.Fatal(err)
					}

					parts = append(parts, part)
				}

				slices.Reverse(parts)
				if output, err = JoinExfil(parts); err != nil || !bytes.Equal(output, input) {
					t.Fatalf("%v reassembly failed: %v", recordType, err)
				}
			}
		}
	}
}

// TestExfilDNSRejectsInvalidPackets covers carrier corruption and DNS format limits.
func TestExfilDNSRejectsInvalidPackets(t *testing.T) {
	var (
		packet  []byte
		message dnsmessage.Message
		err     error
	)

	for _, recordType := range []dnsmessage.Type{dnsmessage.TypeTXT, dnsmessage.TypeA, dnsmessage.TypeAAAA} {
		for _, mutate := range []func(*dnsmessage.Message){
			func(m *dnsmessage.Message) { m.Truncated = true },
			func(m *dnsmessage.Message) { m.Response = false },
			func(m *dnsmessage.Message) { m.RCode = dnsmessage.RCodeNameError },
			func(m *dnsmessage.Message) { m.Answers = m.Answers[1:] },
			func(m *dnsmessage.Message) { m.Answers = m.Answers[:len(m.Answers)-1] },
			func(m *dnsmessage.Message) { m.Answers[0].Header.Class = dnsmessage.ClassCHAOS },
			func(m *dnsmessage.Message) { m.Questions[0].Type = dnsmessage.TypeMX },
			func(m *dnsmessage.Message) { m.Questions = nil },
		} {
			if packet, err = EncodeExfilDNS(make([]byte, 700), "demo.example", recordType, 1); err != nil {
				t.Fatal(err)
			}

			if err = message.Unpack(packet); err != nil {
				t.Fatal(err)
			}

			mutate(&message)
			if packet, err = message.Pack(); err != nil {
				t.Fatal(err)
			}

			if _, err = DecodeExfilDNS(packet); err == nil {
				t.Fatal("accepted invalid DNS carrier packet")
			}
		}
	}

	for _, name := range []string{"", "bad..example", strings.Repeat("x", 64) + ".example"} {
		if _, err = EncodeExfilDNS(make([]byte, 106), name, dnsmessage.TypeTXT, 0); err == nil {
			t.Fatal("accepted invalid DNS name")
		}
	}

	if _, err = EncodeExfilDNS(make([]byte, 106), "demo.example", dnsmessage.TypeMX, 0); err == nil {
		t.Fatal("accepted unsupported carrier")
	}

	if _, err = EncodeExfilDNS(make([]byte, 65535), "demo.example", dnsmessage.TypeA, 0); err == nil {
		t.Fatal("accepted oversized DNS message")
	}
}

// FuzzDecodeExfilDNS exercises malformed DNS input using valid seeds for every carrier.
func FuzzDecodeExfilDNS(f *testing.F) {
	for _, recordType := range []dnsmessage.Type{dnsmessage.TypeTXT, dnsmessage.TypeA, dnsmessage.TypeAAAA} {
		var (
			packet []byte
			err    error
		)

		if packet, err = EncodeExfilDNS(make([]byte, 106), "demo.example", recordType, 0); err != nil {
			f.Fatal(err)
		}

		f.Add(packet)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		_, _ = DecodeExfilDNS(packet)
	})
}
