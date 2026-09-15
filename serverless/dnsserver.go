package serverless

import (
	"encoding/hex"
	"slices"

	"github.com/z46-dev/dnscomms/codec"
	"golang.org/x/net/dns/dnsmessage"
)

// receiveServer makes its DNS decision only when the request reaches the server.
func (n *Network) receiveServer(packet *Packet) {
	var (
		target   string    = packet.Destination
		wire     []byte    = packet.wire
		transfer *Transfer = packet.transfer
		request  codec.ExfilRequest
		err      error
		part     *Part
		outcome  string = "ordinary answer"
		query    dnsmessage.Message
		response []byte
		server   Server
		accepted bool
	)
	for _, candidate := range n.Servers {
		if candidate.ID == target {
			server = candidate
		}
	}
	if packet.transfer == nil && server.Poisoned && server.ForwardTo != "" {
		n.emit(packet, "forwarding ordinary DNS to "+server.ForwardTo)
		packet.via = target
		packet.Source = target
		packet.Destination = server.ForwardTo
		packet.Direction = "upstream"
		n.launch(packet)
		return
	}
	if packet.transfer == nil && n.resolver != nil {
		n.emit(packet, "query received; resolving via Cloudflare DoH")
		n.resolve(packet)
		return
	}
	if request, err = codec.DecodeExfilRequest(wire); err == nil {
		part = &Part{ID: hex.EncodeToString(request.Header.ExfilID[:]), Sequence: request.Header.SeqNum, Total: request.Header.Total, BodyBytes: request.Header.BodySize, Flags: request.Header.Flags}
		accepted = server.Poisoned
	}
	if accepted {
		outcome = "accepted by poisoned server"
	} else if transfer != nil {
		outcome = "normal handling; part not forwarded"
	}
	if err = query.Unpack(wire); err != nil || len(query.Questions) != 1 {
		return
	}
	query.Header.Response = true
	query.Header.RecursionAvailable = true
	query.Additionals = nil
	if transfer == nil || query.Questions[0].Name.String() == "." || accepted {
		var body dnsmessage.ResourceBody
		switch query.Questions[0].Type {
		case dnsmessage.TypeA:
			body = &dnsmessage.AResource{A: [4]byte{192, 0, 2, 42}}
		case dnsmessage.TypeAAAA:
			body = &dnsmessage.AAAAResource{AAAA: [16]byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 42}}
		case dnsmessage.TypeTXT:
			body = &dnsmessage.TXTResource{TXT: []string{"simulated service available"}}
		}
		query.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: query.Questions[0].Name, Type: query.Questions[0].Type, Class: dnsmessage.ClassINET, TTL: 300}, Body: body}}
	} else {
		query.Header.RCode = dnsmessage.RCodeNameError
		outcome = "NXDOMAIN; part not forwarded"
	}
	n.emit(packet, outcome)
	if response, err = query.Pack(); err == nil {
		packet.sequence = request.Header.SeqNum
		n.reply(packet, response)
	}
	if accepted && transfer != nil {
		n.launch(&Packet{Source: target, Destination: "orchestrator", Client: packet.Client, Server: target, Direction: "forward", Classification: "forwarded part", wire: wire, part: part, transfer: transfer})
	}
}

// collect validates identity and sequence before making recovered data available.
func (n *Network) collect(transfer *Transfer, source string, request codec.ExfilRequest) {
	if hex.EncodeToString(request.Header.ExfilID[:]) != transfer.ID || int(request.Header.Total) != transfer.Expected || request.Header.SeqNum >= request.Header.Total || int(request.Header.BodySize) != len(request.Body) {
		transfer.Status = "failed"
		return
	}
	if _, exists := transfer.parts[request.Header.SeqNum]; exists {
		transfer.Status = "failed"
		return
	}
	transfer.parts[request.Header.SeqNum] = request
	transfer.Received = len(transfer.parts)
	if !slices.Contains(transfer.Sources, source) {
		transfer.Sources = append(transfer.Sources, source)
	}
	transfer.Missing = transfer.Missing[:0]
	for sequence := range uint32(transfer.Expected) {
		if _, exists := transfer.parts[sequence]; !exists {
			transfer.Missing = append(transfer.Missing, sequence)
		}
	}
	if transfer.Received == transfer.Expected && transfer.Status != "failed" {
		var (
			parts []codec.ExfilRequest
			data  []byte
			err   error
		)
		for _, part := range transfer.parts {
			parts = append(parts, part)
		}
		if data, err = codec.RebuildExfilData(parts); err != nil {
			transfer.Status = "failed"
		} else {
			transfer.Message = string(data)
			transfer.Status = "complete"
		}
	}
}
