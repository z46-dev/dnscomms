package serverless

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/z46-dev/dnscomms/codec"
	"golang.org/x/net/dns/dnsmessage"
)

const HistoryLimit = 1500

// New creates the topology; an optional resolver replaces offline fixture answers.
func New(resolvers ...Resolver) (network *Network) {
	network = &Network{TrafficFrequency: 0.75, results: make(chan resolutionResult, 16), random: rand.New(rand.NewPCG(1, 2)), Packets: []*Packet{}, Playing: true, Speed: 1, Servers: []Server{{ID: "dns-1", Poisoned: true}, {ID: "dns-2"}, {ID: "dns-3", Poisoned: true}}, Events: []Event{}, Transfers: []*Transfer{}}
	network.context, network.cancel = context.WithCancel(context.Background())
	if len(resolvers) > 0 {
		network.resolver = resolvers[0]
		network.LiveDNS = resolvers[0] != nil
	}
	network.scheduleCover()
	return
}

// Apply validates browser commands before mutating simulation state.
func (n *Network) Apply(input Input) (err error) {
	switch input.Action {
	case "state":
	case "reset":
		n.cancel()
		*n = *New(n.resolver)
	case "configure":
		if n.Active {
			return errors.New("wait for the active transfer before changing servers")
		}
		var servers []Server = input.Servers
		if servers == nil {
			servers = n.Servers
		}
		if err = validateConfiguration(servers, input.TrafficFrequency); err != nil {
			return
		}
		n.Servers = slices.Clone(servers)
		if input.TrafficFrequency != nil {
			n.TrafficFrequency = *input.TrafficFrequency
			n.scheduleCover()
		}
		if input.EvilCover != nil {
			n.EvilCover = *input.EvilCover
		}
	case "send":
		err = n.send(input)
	case "playback":
		if input.Speed != 0.5 && input.Speed != 1 && input.Speed != 2 && input.Speed != 4 {
			return errors.New("choose a playback speed of 0.5, 1, 2, or 4")
		}
		n.Playing, n.Speed = input.Playing, input.Speed
	case "tick":
		if math.IsNaN(input.Delta) || math.IsInf(input.Delta, 0) || input.Delta < 0 || input.Delta > 1 {
			return errors.New("tick must be between zero and one second")
		}
		if n.Playing {
			n.advance(input.Delta * n.Speed)
		}
	default:
		err = errors.New("unknown simulation command")
	}
	return
}

// send prepares real codec packets, preserving earlier captures and assemblies.
func (n *Network) send(input Input) (err error) {
	var (
		types    []dnsmessage.Type
		requests []codec.ExfilRequest
		transfer *Transfer
	)
	if n.Active || len(n.Transfers) >= 32 {
		return errors.New("finish the active transfer, or reset after 32 transfers")
	}
	if len(input.Message) == 0 || len(input.Message) > 16384 || input.Duration < 1 || input.Duration > 120 || math.IsNaN(input.Duration) || math.IsInf(input.Duration, 0) {
		return errors.New("use 1–16384 message bytes and a duration of 1–120 seconds")
	}
	if len(input.Targets) == 0 || len(input.Types) == 0 {
		return errors.New("select at least one target and record type")
	}
	for _, target := range input.Targets {
		if !slices.ContainsFunc(n.Servers, func(server Server) bool { return server.ID == target }) {
			return errors.New("unknown target server")
		}
	}
	for _, name := range input.Types {
		switch name {
		case "A":
			types = append(types, dnsmessage.TypeA)
		case "AAAA":
			types = append(types, dnsmessage.TypeAAAA)
		case "TXT":
			types = append(types, dnsmessage.TypeTXT)
		default:
			return errors.New("choose A, AAAA, or TXT")
		}
	}
	if requests, err = codec.EncodeExfilRequest([]byte(input.Message), codec.ExfilOptions{Servers: input.Targets, Duration: time.Duration(input.Duration * float64(time.Second)), AllowedRecordTypes: types, VaryPayloadLengths: input.Vary}); err != nil {
		return
	}
	transfer = &Transfer{ID: hex.EncodeToString(requests[0].Header.ExfilID[:]), Expected: len(requests), InputBytes: len(input.Message), Status: "sending", Sources: []string{}, Missing: []uint32{}, Requests: []Request{}, parts: make(map[uint32]codec.ExfilRequest)}
	for i, request := range requests {
		var wire []byte
		if wire, err = request.Encode(); err != nil {
			return
		}
		transfer.Requests = append(transfer.Requests, Request{Sequence: request.Header.SeqNum, Target: request.TargetServer, Type: recordName(request.RecordType), Due: n.Time + input.Duration*float64(i+1)/float64(len(requests)), Status: "queued", Bytes: len(wire), wire: wire})
		transfer.Missing = append(transfer.Missing, uint32(i))
	}
	n.Transfers = append(n.Transfers, transfer)
	n.Active = true
	return
}

const (
	TickDuration = 0.02
	HopDuration  = 0.8
)

// advance runs fixed game ticks so packet delivery is independent of frame rate.
func (n *Network) advance(delta float64) {
	n.remainder += delta
	for n.remainder+1e-9 >= TickDuration {
		n.remainder -= TickDuration
		n.ticks++
		n.Time = float64(n.ticks) * TickDuration
		n.updateResolutions()
		n.updatePackets()
		n.updateClients()
		n.updateTransfers()
	}
}

// updateClients releases due requests; receivers will handle them on a later tick.
func (n *Network) updateClients() {
	for n.TrafficFrequency > 0 && n.Time+1e-9 >= n.nextCover {
		n.cover()
		n.ordinary++
		n.scheduleCover()
	}
	for _, transfer := range n.Transfers {
		for transfer.Sent < transfer.Expected && transfer.Requests[transfer.Sent].Due <= n.Time+1e-9 {
			var request *Request = &transfer.Requests[transfer.Sent]
			n.route("exfil-client", request.Target, request.wire, transfer)
			request.Status = "in flight"
			transfer.Sent++
		}
	}
}

// scheduleCover chooses the next background query from a Poisson process.
func (n *Network) scheduleCover() {
	if n.TrafficFrequency <= 0 {
		n.nextCover = math.Inf(1)
	} else {
		n.nextCover = n.Time + n.random.ExpFloat64()/n.TrafficFrequency
	}
}

// updateTransfers keeps topology locked until requests, replies and forwards settle.
func (n *Network) updateTransfers() {
	n.Active = false
	for _, transfer := range n.Transfers {
		var pending bool = transfer.Sent < transfer.Expected || slices.ContainsFunc(n.Packets, func(packet *Packet) bool { return packet.transfer == transfer })
		n.Active = n.Active || pending
		if !pending && transfer.Status == "sending" {
			transfer.Status = "incomplete"
		}
	}
}

func recordName(record dnsmessage.Type) (name string) {
	name = strings.TrimPrefix(record.String(), "Type")
	return
}
