package serverless

import (
	"encoding/hex"
	"slices"

	"github.com/z46-dev/dnscomms/codec"
)

// route places a request on the client-to-firewall link without invoking its receiver.
func (n *Network) route(client, target string, wire []byte, transfer *Transfer) {
	var packet *Packet = &Packet{Source: client, Destination: "firewall", Client: client, Server: target, Direction: "outbound", Classification: "ordinary DNS", wire: wire, transfer: transfer}
	if transfer != nil {
		var (
			request codec.ExfilRequest
			err     error
		)
		packet.Classification = "exfil part"
		if request, err = codec.DecodeExfilRequest(wire); err == nil {
			packet.part = &Part{ID: hex.EncodeToString(request.Header.ExfilID[:]), Sequence: request.Header.SeqNum, Total: request.Header.Total, BodyBytes: request.Header.BodySize, Flags: request.Header.Flags}
		}
	}
	n.launch(packet)
}

func (n *Network) launch(packet *Packet) {
	if packet.ID == 0 {
		n.nextPacket++
		packet.ID = n.nextPacket
	}
	packet.Progress = 0
	n.Packets = append(n.Packets, packet)
}

// updatePackets moves each packet once and delivers arrivals after movement finishes.
func (n *Network) updatePackets() {
	var arrivals []*Packet
	n.Packets = slices.DeleteFunc(n.Packets, func(packet *Packet) bool {
		packet.Progress = min(1, packet.Progress+TickDuration/HopDuration)
		if packet.Progress >= 1-1e-9 {
			arrivals = append(arrivals, packet)
			return true
		}
		return false
	})
	for _, packet := range arrivals {
		n.receive(packet)
	}
}

// receive dispatches one delivery to the firewall, DNS server, client or orchestrator.
func (n *Network) receive(packet *Packet) {
	switch packet.Destination {
	case "firewall":
		n.emit(packet, "received by firewall; passed onward")
		packet.Source = "firewall"
		packet.Destination = packet.Server
		if packet.Direction == "inbound" {
			packet.Destination = packet.Client
		}
		n.launch(packet)
	case "orchestrator":
		var (
			request codec.ExfilRequest
			err     error
		)
		n.emit(packet, "delivered to orchestrator")
		if request, err = codec.DecodeExfilRequest(packet.wire); err == nil && packet.transfer != nil {
			n.collect(packet.transfer, packet.Server, request)
		}
	case "exfil-client", "client-1", "client-2":
		n.emit(packet, "reply received by client")
		if packet.transfer != nil {
			packet.transfer.Requests[packet.sequence].Status = "reply received"
		}
	default:
		if slices.ContainsFunc(n.Servers, func(server Server) bool { return server.ID == packet.Destination }) {
			if packet.Direction == "downstream" {
				n.emit(packet, "upstream reply received; returning through firewall")
				packet.Source = packet.Destination
				packet.Destination = "firewall"
				packet.Direction = "inbound"
				n.launch(packet)
			} else {
				n.receiveServer(packet)
			}
		} else {
			n.emit(packet, "dropped: server removed")
		}
	}
}
