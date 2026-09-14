package codec

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ExfilVersion        byte = 1
	ExfilHeaderSize          = 106
	MaxExfilMessageSize      = 16 << 20
	MaxExfilParts            = 65536
	exfilEncrypted      byte = 1
)

type (
	RequestID [32]byte
	SourceID  [16]byte

	// ExfilOptions controls framing; Key optionally enables AES-256-GCM.
	ExfilOptions struct {
		Source   SourceID
		PartSize int // Maximum binary frame size, including header and encryption overhead.
		Key      []byte
	}

	ExfilHeader struct {
		Version   byte
		Flags     byte
		Request   RequestID
		Source    SourceID
		CreatedAt int64  // Unix milliseconds; informational, not used for uniqueness.
		Index     uint32 // Zero-based.
		Parts     uint32
		Size      uint32   // Original message size.
		Digest    [32]byte // SHA-256 of the original message.
	}

	ExfilPart struct {
		Header  ExfilHeader
		Payload []byte // Plaintext after successful decoding.
	}
)

// EncodeExfil splits a message into independent binary frames without performing I/O.
func EncodeExfil(data []byte, options ExfilOptions) (frames [][]byte, err error) {
	var (
		header   ExfilHeader
		aead     cipher.AEAD
		capacity int
	)

	if len(data) > MaxExfilMessageSize {
		err = errors.New("message exceeds exfil size limit")
		return
	}

	if aead, err = exfilCipher(options.Key); err != nil {
		return
	}

	if options.PartSize <= ExfilHeaderSize {
		err = errors.New("part size must exceed header size")
		return
	}

	capacity = options.PartSize - ExfilHeaderSize
	if aead != nil {
		capacity -= aead.NonceSize() + aead.Overhead()
		header.Flags = exfilEncrypted
	}

	if capacity <= 0 {
		err = errors.New("part size must leave room for payload")
		return
	}

	header.Version = ExfilVersion
	header.Source = options.Source
	header.CreatedAt = time.Now().UnixMilli()
	header.Parts = uint32(1 + max(0, len(data)-1)/capacity)
	header.Size = uint32(len(data))
	header.Digest = sha256.Sum256(data)
	if header.Parts > MaxExfilParts {
		err = errors.New("message exceeds exfil part limit")
		return
	}

	if _, err = rand.Read(header.Request[:]); err != nil {
		return
	}

	frames = make([][]byte, 0, header.Parts)
	for index := uint32(0); index < header.Parts; index++ {
		var (
			start   int    = int(index) * capacity
			payload []byte = data[start:min(start+capacity, len(data))]
			frame   []byte
		)

		header.Index = index
		frame = marshalExfilHeader(header)
		if aead != nil {
			var nonce []byte = make([]byte, aead.NonceSize())
			if _, err = rand.Read(nonce); err != nil {
				frames = nil
				return
			}

			frame = append(frame, nonce...)
			frame = aead.Seal(frame, nonce, payload, frame[:ExfilHeaderSize])
		} else {
			frame = append(frame, payload...)
		}

		frames = append(frames, frame)
	}

	return
}

// DecodeExfil validates a frame and decrypts its payload when encryption is enabled.
func DecodeExfil(frame, key []byte) (part ExfilPart, err error) {
	var aead cipher.AEAD
	if len(frame) < ExfilHeaderSize || len(frame) > ExfilHeaderSize+MaxExfilMessageSize+28 {
		err = errors.New("invalid exfil frame size")
		return
	}

	if string(frame[:4]) != "DXFL" {
		err = errors.New("invalid exfil magic")
		return
	}

	part.Header.Version = frame[4]
	part.Header.Flags = frame[5]
	copy(part.Header.Request[:], frame[6:38])
	copy(part.Header.Source[:], frame[38:54])
	part.Header.CreatedAt = int64(binary.BigEndian.Uint64(frame[54:62]))
	part.Header.Index = binary.BigEndian.Uint32(frame[62:66])
	part.Header.Parts = binary.BigEndian.Uint32(frame[66:70])
	part.Header.Size = binary.BigEndian.Uint32(frame[70:74])
	copy(part.Header.Digest[:], frame[74:106])

	if err = validateExfilHeader(part.Header); err != nil {
		return
	}

	if aead, err = exfilCipher(key); err != nil {
		return
	}

	if part.Header.Flags == exfilEncrypted {
		if aead == nil || len(frame) < ExfilHeaderSize+aead.NonceSize()+aead.Overhead() {
			err = errors.New("encrypted frame requires a key and complete ciphertext")
			return
		}

		if part.Payload, err = aead.Open(nil, frame[ExfilHeaderSize:ExfilHeaderSize+aead.NonceSize()], frame[ExfilHeaderSize+aead.NonceSize():], frame[:ExfilHeaderSize]); err != nil {
			return
		}
	} else if aead != nil {
		err = errors.New("expected encrypted frame")
		return
	} else {
		part.Payload = bytes.Clone(frame[ExfilHeaderSize:])
	}

	if len(part.Payload) > int(part.Header.Size) || (part.Header.Size > 0 && len(part.Payload) == 0) {
		err = errors.New("invalid exfil payload size")
		part.Payload = nil
	}

	return
}

// JoinExfil reassembles one request in any order, tolerating identical duplicate parts.
func JoinExfil(parts []ExfilPart) (data []byte, err error) {
	var (
		header   ExfilHeader
		payloads map[uint32][]byte
		total    int
	)

	if len(parts) == 0 || len(parts) > 2*MaxExfilParts {
		err = errors.New("invalid number of input parts")
		return
	}

	header = parts[0].Header
	header.Index = 0
	if err = validateExfilHeader(header); err != nil {
		return
	}

	payloads = make(map[uint32][]byte)
	for _, part := range parts {
		var (
			metadata ExfilHeader = part.Header
			existing []byte
			ok       bool
		)

		metadata.Index = 0
		if metadata != header || part.Header.Index >= header.Parts {
			err = errors.New("parts belong to different requests, sources, or metadata")
			return
		}

		if existing, ok = payloads[part.Header.Index]; ok {
			if !bytes.Equal(existing, part.Payload) {
				err = errors.New("conflicting duplicate part")
				return
			}

			continue
		}

		if len(part.Payload) > int(header.Size)-total || (header.Size > 0 && len(part.Payload) == 0) {
			err = errors.New("invalid assembled payload size")
			return
		}

		total += len(part.Payload)
		payloads[part.Header.Index] = part.Payload
	}

	if len(payloads) != int(header.Parts) || total != int(header.Size) {
		err = errors.New("incomplete message")
		return
	}

	data = make([]byte, 0, total)
	for index := uint32(0); index < header.Parts; index++ {
		data = append(data, payloads[index]...)
	}

	if sha256.Sum256(data) != header.Digest {
		err = errors.New("message digest mismatch")
		data = nil
	}

	return
}

// EncodeExfilText returns case-insensitive, unpadded base32 for a DNS transport adapter.
func EncodeExfilText(frame []byte) (encoded string) {
	encoded = strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(frame))
	return
}

// DecodeExfilText decodes base32; the caller subsequently validates the binary frame.
func DecodeExfilText(encoded string) (frame []byte, err error) {
	if len(encoded) > base32.StdEncoding.WithPadding(base32.NoPadding).EncodedLen(ExfilHeaderSize+MaxExfilMessageSize+28) {
		err = errors.New("encoded frame exceeds size limit")
		return
	}

	frame, err = base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(encoded))
	return
}

// exfilCipher constructs the optional authenticated payload cipher.
func exfilCipher(key []byte) (aead cipher.AEAD, err error) {
	var block cipher.Block
	if len(key) == 0 {
		return
	}

	if len(key) != 32 {
		err = errors.New("exfil key must be exactly 32 bytes")
		return
	}

	if block, err = aes.NewCipher(key); err == nil {
		aead, err = cipher.NewGCM(block)
	}

	return
}

// validateExfilHeader rejects unsupported features and impossible multipart metadata.
func validateExfilHeader(header ExfilHeader) (err error) {
	if header.Version != ExfilVersion || header.Flags > exfilEncrypted {
		err = fmt.Errorf("unsupported exfil version or flags: %d/%d", header.Version, header.Flags)
	} else if header.Size > MaxExfilMessageSize || header.Parts == 0 || header.Parts > MaxExfilParts || header.Index >= header.Parts {
		err = errors.New("invalid exfil multipart metadata")
	} else if (header.Size == 0 && header.Parts != 1) || (header.Size > 0 && header.Parts > header.Size) {
		err = errors.New("part count is inconsistent with message size")
	}

	return
}

// marshalExfilHeader writes the fixed-width, network-byte-order version 1 header.
func marshalExfilHeader(header ExfilHeader) (frame []byte) {
	frame = make([]byte, ExfilHeaderSize)
	copy(frame[:4], "DXFL")
	frame[4] = header.Version
	frame[5] = header.Flags
	copy(frame[6:38], header.Request[:])
	copy(frame[38:54], header.Source[:])
	binary.BigEndian.PutUint64(frame[54:62], uint64(header.CreatedAt))
	binary.BigEndian.PutUint32(frame[62:66], header.Index)
	binary.BigEndian.PutUint32(frame[66:70], header.Parts)
	binary.BigEndian.PutUint32(frame[70:74], header.Size)
	copy(frame[74:106], header.Digest[:])
	return
}
