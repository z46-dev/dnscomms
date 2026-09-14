package codec

// Here, we can create an "exfil builder" that can take arbitrary data (len > 0) and can encode and send
// the data servers (len servers > 0). There are a few parts to the encoder:
// - the header portion (exfil ID, sequence number, total number of packets, etc.)
// - the payload portion (the actual data being sent)
// - the footer portion (checksum, etc.)

type (
	RequestHeader struct {}

	ExfilBuilder struct {}
)
