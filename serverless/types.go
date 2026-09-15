package serverless

import (
	"context"
	"math/rand/v2"

	"github.com/z46-dev/dnscomms/codec"
)

type (
	Server struct {
		ForwardTo string `json:"forwardTo"`
		ID        string `json:"id"`
		Poisoned  bool   `json:"poisoned"`
	}

	Input struct {
		TrafficFrequency *float64 `json:"trafficFrequency,omitempty"`
		EvilCover        *bool    `json:"evilCover,omitempty"`
		Action           string   `json:"action"`
		Servers          []Server `json:"servers"`
		Message          string   `json:"message"`
		Targets          []string `json:"targets"`
		Types            []string `json:"types"`
		Duration         float64  `json:"duration"`
		Vary             bool     `json:"vary"`
		Delta            float64  `json:"delta"`
		Playing          bool     `json:"playing"`
		Speed            float64  `json:"speed"`
	}

	Part struct {
		ID        string `json:"id"`
		Sequence  uint32 `json:"sequence"`
		Total     uint32 `json:"total"`
		BodyBytes uint32 `json:"bodyBytes"`
		Flags     byte   `json:"flags"`
	}

	Event struct {
		TrafficLabel   string  `json:"trafficLabel,omitempty"`
		PacketID       int     `json:"packetId"`
		Client         string  `json:"client"`
		Server         string  `json:"server"`
		ID             int     `json:"id"`
		Time           float64 `json:"time"`
		Source         string  `json:"source"`
		Destination    string  `json:"destination"`
		Direction      string  `json:"direction"`
		Type           string  `json:"type"`
		Name           string  `json:"name"`
		Classification string  `json:"classification"`
		Outcome        string  `json:"outcome"`
		Bytes          int     `json:"bytes"`
		Hex            string  `json:"hex"`
		DNS            any     `json:"dns"`
		Part           *Part   `json:"part,omitempty"`
	}

	Request struct {
		Sequence uint32  `json:"sequence"`
		Target   string  `json:"target"`
		Type     string  `json:"type"`
		Due      float64 `json:"due"`
		Status   string  `json:"status"`
		Bytes    int     `json:"bytes"`
		wire     []byte
	}

	Transfer struct {
		ID         string    `json:"id"`
		Expected   int       `json:"expected"`
		Received   int       `json:"received"`
		Sent       int       `json:"sent"`
		Missing    []uint32  `json:"missing"`
		Sources    []string  `json:"sources"`
		Status     string    `json:"status"`
		Message    string    `json:"message"`
		InputBytes int       `json:"inputBytes"`
		Requests   []Request `json:"requests"`
		parts      map[uint32]codec.ExfilRequest
	}

	Packet struct {
		TrafficLabel   string `json:"trafficLabel,omitempty"`
		via            string
		ID             int     `json:"id"`
		Source         string  `json:"source"`
		Destination    string  `json:"destination"`
		Client         string  `json:"client"`
		Server         string  `json:"server"`
		Direction      string  `json:"direction"`
		Classification string  `json:"classification"`
		Progress       float64 `json:"progress"`
		sequence       uint32
		wire           []byte
		part           *Part
		transfer       *Transfer
	}

	Network struct {
		TrafficFrequency float64 `json:"trafficFrequency"`
		EvilCover        bool    `json:"evilCover"`
		LiveDNS          bool    `json:"liveDNS"`
		Resolving        int     `json:"resolving"`
		resolver         Resolver
		results          chan resolutionResult
		cancel           context.CancelFunc
		context          context.Context
		nextCover        float64
		random           *rand.Rand
		Packets          []*Packet `json:"packets"`
		ticks            int
		remainder        float64
		nextPacket       int
		Time             float64     `json:"time"`
		Playing          bool        `json:"playing"`
		Speed            float64     `json:"speed"`
		Servers          []Server    `json:"servers"`
		Events           []Event     `json:"events"`
		Transfers        []*Transfer `json:"transfers"`
		Active           bool        `json:"active"`
		Dropped          int         `json:"dropped"`
		nextID           int
		ordinary         int
	}
)
