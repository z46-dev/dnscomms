package codec

import (
	"bytes"
	"testing"
)

// TestExfilRoundTrip covers binary data, encryption, text transport, ordering, and duplicates.
func TestExfilRoundTrip(t *testing.T) {
	for _, key := range [][]byte{nil, bytes.Repeat([]byte{42}, 32)} {
		for _, input := range [][]byte{nil, []byte("hello"), bytes.Repeat([]byte{0, 1, 127, 255}, 1000)} {
			var (
				frames [][]byte
				parts  []ExfilPart
				frame  []byte
				part   ExfilPart
				output []byte
				err    error
			)

			if frames, err = EncodeExfil(input, ExfilOptions{Source: SourceID{1}, PartSize: 200, Key: key}); err != nil {
				t.Fatal(err)
			}

			for index := len(frames) - 1; index >= 0; index-- {
				if len(frames[index]) > 200 {
					t.Fatal("frame exceeded part size")
				}

				if frame, err = DecodeExfilText(EncodeExfilText(frames[index])); err != nil {
					t.Fatal(err)
				}

				if part, err = DecodeExfil(frame, key); err != nil {
					t.Fatal(err)
				}

				parts = append(parts, part)
			}

			parts = append(parts, parts[0])
			if output, err = JoinExfil(parts); err != nil || !bytes.Equal(output, input) {
				t.Fatalf("round trip failed: %v", err)
			}
		}
	}
}

// TestExfilRejectsCorruption covers incomplete, mixed, conflicting, and tampered messages.
func TestExfilRejectsCorruption(t *testing.T) {
	var (
		frames [][]byte
		parts  []ExfilPart
		part   ExfilPart
		err    error
	)

	if frames, err = EncodeExfil(bytes.Repeat([]byte{1}, 100), ExfilOptions{PartSize: 156}); err != nil {
		t.Fatal(err)
	}

	for _, frame := range frames {
		if part, err = DecodeExfil(frame, nil); err != nil {
			t.Fatal(err)
		}

		parts = append(parts, part)
	}

	if _, err = JoinExfil(parts[:1]); err == nil {
		t.Fatal("accepted incomplete message")
	}

	for _, mutate := range []func(*ExfilPart){
		func(p *ExfilPart) { p.Header.Source[0]++ },
		func(p *ExfilPart) { p.Header.Request[0]++ },
		func(p *ExfilPart) { p.Header.Parts++ },
		func(p *ExfilPart) { p.Payload[0]++ },
	} {
		var changed []ExfilPart = append([]ExfilPart(nil), parts...)
		changed[0].Payload = bytes.Clone(changed[0].Payload)
		mutate(&changed[0])
		if _, err = JoinExfil(changed); err == nil {
			t.Fatal("accepted corrupted message")
		}
	}

	part = parts[0]
	part.Payload = bytes.Clone(part.Payload)
	part.Payload[0]++
	if _, err = JoinExfil(append(parts, part)); err == nil {
		t.Fatal("accepted conflicting duplicate")
	}

	if frames, err = EncodeExfil([]byte("secret"), ExfilOptions{PartSize: 200, Key: make([]byte, 32)}); err != nil {
		t.Fatal(err)
	}

	for _, offset := range []int{38, ExfilHeaderSize, len(frames[0]) - 1} {
		var corrupted []byte = bytes.Clone(frames[0])
		corrupted[offset] ^= 1
		if _, err = DecodeExfil(corrupted, make([]byte, 32)); err == nil {
			t.Fatal("accepted unauthenticated frame")
		}
	}

	if _, err = DecodeExfil(frames[0], nil); err == nil {
		t.Fatal("accepted encrypted frame without key")
	}

	if _, err = DecodeExfil(frames[0], bytes.Repeat([]byte{1}, 32)); err == nil {
		t.Fatal("accepted wrong key")
	}
}

// FuzzDecodeExfil checks that arbitrary wire data cannot panic the decoder.
func FuzzDecodeExfil(f *testing.F) {
	var (
		frames [][]byte
		err    error
	)

	if frames, err = EncodeExfil([]byte("seed"), ExfilOptions{PartSize: 200}); err != nil {
		f.Fatal(err)
	}

	f.Add(frames[0])
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, frame []byte) {
		var (
			part ExfilPart
			err  error
		)

		if part, err = DecodeExfil(frame, nil); err == nil {
			_, _ = JoinExfil([]ExfilPart{part})
		}
	})
}

// TestExfilLimits exercises configuration boundaries and request ID freshness.
func TestExfilLimits(t *testing.T) {
	var (
		first, second [][]byte
		err           error
	)

	for _, options := range []ExfilOptions{
		{PartSize: -1},
		{PartSize: ExfilHeaderSize},
		{PartSize: ExfilHeaderSize + 28, Key: make([]byte, 32)},
		{PartSize: 200, Key: make([]byte, 31)},
	} {
		if _, err = EncodeExfil([]byte("test"), options); err == nil {
			t.Fatal("accepted invalid options")
		}
	}

	if _, err = EncodeExfil(make([]byte, MaxExfilMessageSize+1), ExfilOptions{PartSize: 200}); err == nil {
		t.Fatal("accepted oversized message")
	}

	if _, err = EncodeExfil(make([]byte, MaxExfilParts+1), ExfilOptions{PartSize: ExfilHeaderSize + 1}); err == nil {
		t.Fatal("accepted too many parts")
	}

	if first, err = EncodeExfil([]byte("test"), ExfilOptions{PartSize: int(^uint(0) >> 1)}); err != nil {
		t.Fatal(err)
	}

	if second, err = EncodeExfil([]byte("test"), ExfilOptions{PartSize: 200}); err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(first[0][6:38], second[0][6:38]) {
		t.Fatal("request ID was reused")
	}
}
