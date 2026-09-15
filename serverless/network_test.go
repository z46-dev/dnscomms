package serverless

import (
	"encoding/hex"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/z46-dev/dnscomms/codec"
	"golang.org/x/net/dns/dnsmessage"
)

// TestTraffic exercises real packets, complete and incomplete routes, and event ordering.
func TestTraffic(t *testing.T) {
	for _, record := range []string{"A", "AAAA", "TXT"} {
		for _, target := range []string{"dns-1", "dns-2"} {
			t.Run(record+"/"+target, func(t *testing.T) {
				var (
					network  *Network = New()
					message  string   = strings.Repeat("Hello 世界 ", 40)
					transfer *Transfer
					lastTime float64
					err      error
				)
				apply(t, network, Input{Action: "send", Message: message, Targets: []string{target}, Types: []string{record}, Duration: 3, Vary: true})
				if err = network.Apply(Input{Action: "configure", Servers: network.Servers}); err == nil {
					t.Fatal("active topology mutation accepted")
				}
				apply(t, network, Input{Action: "tick", Delta: 1})
				transfer = network.Transfers[0]
				if transfer.Message != "" || !network.Active {
					t.Fatal("premature assembly")
				}
				apply(t, network, Input{Action: "tick", Delta: 1})
				apply(t, network, Input{Action: "tick", Delta: 1})
				for range 4 {
					apply(t, network, Input{Action: "tick", Delta: 1})
				}
				if network.Active || transfer.Sent != transfer.Expected {
					t.Fatal("transfer did not finish")
				}
				if target == "dns-1" {
					if transfer.Message != message || transfer.Status != "complete" || len(transfer.Missing) != 0 {
						t.Fatal("poisoned route failed")
					}
				} else if transfer.Message != "" || transfer.Status != "incomplete" || len(transfer.Missing) != transfer.Expected {
					t.Fatal("normal route recovered data")
				}
				var ordinary int
				for i, event := range network.Events {
					var (
						wire    []byte
						message dnsmessage.Message
					)
					if event.ID != i+1 || event.Time < lastTime {
						t.Fatal("capture ordering")
					}
					lastTime = event.Time
					if wire, err = hex.DecodeString(event.Hex); err != nil {
						t.Fatal(err)
					}
					if err = message.Unpack(wire); err != nil {
						t.Fatal(err)
					}
					if event.Bytes != len(wire) {
						t.Fatal("wire length")
					}
					if event.Classification == "ordinary DNS" {
						ordinary++
					}
					if event.Direction == "inbound" && (!message.Response || len(message.Questions) != 1) {
						t.Fatal("invalid reply")
					}
				}
				if ordinary == 0 {
					t.Fatal("cover traffic did not run")
				}
				apply(t, network, Input{Action: "send", Message: "next", Targets: []string{"dns-1"}, Types: []string{record}, Duration: 1})
				if network.Transfers[1].ID == transfer.ID || len(network.Events) == 0 {
					t.Fatal("history erased or ID reused")
				}
			})
		}
	}
}

// TestPlaybackAndBounds verifies deterministic cover traffic, pause, reset and bounded history.
func TestPlaybackAndBounds(t *testing.T) {
	var (
		first    *Network = New()
		second   *Network = New()
		snapshot int
	)
	for range 12 {
		apply(t, first, Input{Action: "tick", Delta: 0.5})
	}
	for range 6 {
		apply(t, second, Input{Action: "tick", Delta: 1})
	}
	if !reflect.DeepEqual(first.Events, second.Events) || !reflect.DeepEqual(first.Packets, second.Packets) {
		t.Fatal("cover traffic depends on tick size")
	}
	for _, event := range first.Events {
		if event.Direction == "inbound" {
			var (
				message dnsmessage.Message
				wire    []byte
				err     error
			)
			if wire, err = hex.DecodeString(event.Hex); err != nil {
				t.Fatal(err)
			}
			if err = message.Unpack(wire); err != nil {
				t.Fatal(err)
			}
			if len(message.Answers) != 1 || message.RCode != dnsmessage.RCodeSuccess {
				t.Fatal("cover reply must answer")
			}
		}
	}
	snapshot = len(first.Events)
	apply(t, first, Input{Action: "playback", Playing: false, Speed: 2})
	apply(t, first, Input{Action: "tick", Delta: 1})
	if first.Time != 6 || len(first.Events) != snapshot {
		t.Fatal("pause changed state")
	}
	apply(t, first, Input{Action: "playback", Playing: true, Speed: 4})
	for range 200 {
		apply(t, first, Input{Action: "tick", Delta: 1})
	}
	if len(first.Events) != HistoryLimit || first.Dropped == 0 {
		t.Fatal("unbounded event log")
	}
	apply(t, first, Input{Action: "reset"})
	if first.Time != 0 || len(first.Events) != 0 || len(first.Transfers) != 0 {
		t.Fatal("reset retained state")
	}
}

// TestRejectedParts checks assembler identity validation and ordinary parser fallback.
func TestRejectedParts(t *testing.T) {
	var (
		network *Network = New()
		request codec.ExfilRequest
		err     error
		wire    []byte
		message dnsmessage.Message = dnsmessage.Message{Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName("www.example.com."), Type: dnsmessage.TypeTXT, Class: dnsmessage.ClassINET}}}
	)
	apply(t, network, Input{Action: "send", Message: "message", Targets: []string{"dns-1"}, Types: []string{"TXT"}, Duration: 1})
	if request, err = codec.DecodeExfilRequest(network.Transfers[0].Requests[0].wire); err != nil {
		t.Fatal(err)
	}
	request.Header.Total++
	network.collect(network.Transfers[0], "dns-1", request)
	if network.Transfers[0].Status != "failed" || network.Transfers[0].Message != "" {
		t.Fatal("invalid total assembled")
	}
	if wire, err = message.Pack(); err != nil {
		t.Fatal(err)
	}
	network = New()
	network.route("client-1", "dns-1", wire, nil)
	for range 4 {
		apply(t, network, Input{Action: "tick", Delta: 1})
	}
	if !slices.ContainsFunc(network.Events, func(event Event) bool {
		return event.Destination == "client-1" && event.Classification == "cover response"
	}) {
		t.Fatal("failed parse did not fall back to DNS")
	}
}

func apply(t *testing.T, network *Network, input Input) {
	t.Helper()
	var err error
	if err = network.Apply(input); err != nil {
		t.Fatal(err)
	}
}

// TestMixedRouting retains received parts when one request reaches a normal server.
func TestMixedRouting(t *testing.T) {
	var network *Network = New()
	apply(t, network, Input{Action: "send", Message: strings.Repeat("x", 400), Targets: []string{"dns-1"}, Types: []string{"A"}, Duration: 1})
	network.Transfers[0].Requests[1].Target = "dns-2"
	for range 5 {
		apply(t, network, Input{Action: "tick", Delta: 1})
	}
	if network.Transfers[0].Status != "incomplete" || network.Transfers[0].Received != 2 || !reflect.DeepEqual(network.Transfers[0].Missing, []uint32{1}) || network.Transfers[0].Message != "" {
		t.Fatal("misrouted part was recovered or received parts were discarded")
	}
}

// TestValidation rejects invalid browser input without changing topology or queue.
func TestValidation(t *testing.T) {
	var network *Network = New()
	for _, input := range []Input{
		{Action: "configure", Servers: []Server{}},
		{Action: "configure", Servers: []Server{{ID: "dns-7", Poisoned: true}}},
		{Action: "send", Message: "hello", Duration: 1, Targets: []string{"dns-99"}, Types: []string{"A"}},
		{Action: "send", Message: "hello", Duration: 1, Targets: []string{"dns-1"}, Types: []string{"MX"}},
		{Action: "send", Message: strings.Repeat("x", 16385), Duration: 1, Targets: []string{"dns-1"}, Types: []string{"A"}},
		{Action: "playback", Speed: 100},
		{Action: "tick", Delta: -1},
	} {
		var err error
		if err = network.Apply(input); err == nil {
			t.Fatalf("accepted invalid command: %s", input.Action)
		}
	}
	if len(network.Transfers) != 0 || len(network.Servers) != 3 || network.Active {
		t.Fatal("invalid input changed state")
	}
}

// TestHopDelivery prevents decisions, responses or reconstruction before physical arrival.
func TestHopDelivery(t *testing.T) {
	var (
		network   *Network = New()
		frequency float64
		transfer  *Transfer
		progress  float64
	)
	apply(t, network, Input{Action: "configure", TrafficFrequency: &frequency})
	apply(t, network, Input{Action: "send", Message: "on arrival", Targets: []string{"dns-1"}, Types: []string{"A"}, Duration: 1})
	transfer = network.Transfers[0]
	apply(t, network, Input{Action: "tick", Delta: 1})
	if len(network.Events) != 0 || len(network.Packets) != 1 || network.Packets[0].Progress != 0 {
		t.Fatal("launch delivered a packet immediately")
	}
	apply(t, network, Input{Action: "tick", Delta: 0.4})
	progress = network.Packets[0].Progress
	if progress < 0.49 || progress > 0.51 || len(network.Events) != 0 {
		t.Fatal("packet must be halfway to the firewall without a receive event")
	}
	apply(t, network, Input{Action: "playback", Playing: false, Speed: 1})
	apply(t, network, Input{Action: "tick", Delta: 1})
	if network.Packets[0].Progress != progress {
		t.Fatal("pause moved a packet")
	}
	apply(t, network, Input{Action: "playback", Playing: true, Speed: 1})
	apply(t, network, Input{Action: "tick", Delta: 0.4})
	if len(network.Events) != 1 || network.Events[0].Destination != "firewall" || transfer.Received != 0 {
		t.Fatal("firewall must receive before server handling")
	}
	apply(t, network, Input{Action: "tick", Delta: HopDuration})
	if transfer.Received != 0 || !slices.ContainsFunc(network.Packets, func(packet *Packet) bool { return packet.Destination == "orchestrator" && packet.Progress == 0 }) {
		t.Fatal("server must launch a forward without assembling immediately")
	}
	apply(t, network, Input{Action: "tick", Delta: HopDuration})
	if transfer.Received != 1 || transfer.Message != "on arrival" || !network.Active {
		t.Fatal("orchestrator must assemble on arrival while the reply is still in flight")
	}
	apply(t, network, Input{Action: "tick", Delta: HopDuration})
	if network.Active || transfer.Requests[0].Status != "reply received" || transfer.Received != 1 {
		t.Fatal("client must receive its reply once before the transfer settles")
	}
}

// TestTrafficConfiguration checks frequency, cover from the exfil client and upstream constraints.
func TestTrafficConfiguration(t *testing.T) {
	var (
		network   *Network = New()
		frequency float64  = 8
		evil      bool     = true
		err       error
	)
	apply(t, network, Input{Action: "configure", TrafficFrequency: &frequency, EvilCover: &evil})
	apply(t, network, Input{Action: "tick", Delta: 1})
	if len(network.Packets) < 2 || !slices.ContainsFunc(network.Packets, func(packet *Packet) bool { return packet.Client == "exfil-client" && packet.transfer == nil }) {
		t.Fatal("random cover from the exfil client was not scheduled")
	}
	frequency = 0
	apply(t, network, Input{Action: "configure", TrafficFrequency: &frequency})
	for range 5 {
		apply(t, network, Input{Action: "tick", Delta: 1})
	}
	if len(network.Packets) != 0 {
		t.Fatal("traffic did not stop")
	}
	if err = network.Apply(Input{Action: "configure", Servers: []Server{{ID: "dns-1", Poisoned: true, ForwardTo: "dns-1"}}}); err == nil {
		t.Fatal("accepted a forwarding loop")
	}
	apply(t, network, Input{Action: "configure", Servers: []Server{{ID: "dns-1"}}})
	if network.Servers[0].Poisoned {
		t.Fatal("zero exfil servers must be allowed")
	}
}

// TestTrafficCorpus protects the embedded sample's promised size, ratio and DNS shape.
func TestTrafficCorpus(t *testing.T) {
	var good, bad int
	if len(traffic) != 1000 {
		t.Fatal("expected 1000 embedded traffic rows")
	}
	for _, row := range traffic {
		var err error
		if _, err = dnsmessage.NewName(row.domain); err != nil {
			t.Fatal(err)
		}
		if row.record != dnsmessage.TypeA && row.record != dnsmessage.TypeAAAA && row.record != dnsmessage.TypeTXT {
			t.Fatal("invalid query type")
		}
		switch row.label {
		case "good":
			good++
		case "bad":
			bad++
		default:
			t.Fatal("invalid traffic label")
		}
	}
	if good != 900 || bad != 100 {
		t.Fatal("expected a 90/10 label ratio")
	}
}
