package serverless

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

type resolverTransport func(*http.Request) (*http.Response, error)

func (transport resolverTransport) RoundTrip(request *http.Request) (response *http.Response, err error) {
	response, err = transport(request)
	return
}

// TestCloudflareTransport exercises the real Go HTTP adapter with controlled wire responses.
func TestCloudflareTransport(t *testing.T) {
	var (
		original  *http.Client = http.DefaultClient
		malformed bool
		err       error
		response  []byte
	)
	t.Cleanup(func() { http.DefaultClient = original })
	http.DefaultClient = &http.Client{Transport: resolverTransport(func(request *http.Request) (response *http.Response, err error) {
		var (
			wire  []byte
			query dnsmessage.Message
		)
		if request.URL.Host != "cloudflare-dns.com" || request.Header.Get("Accept") != "application/dns-message" {
			t.Error("incorrect DoH endpoint or media type")
		}
		if wire, err = base64.RawURLEncoding.DecodeString(request.URL.Query().Get("dns")); err != nil {
			return
		}
		if err = query.Unpack(wire); err != nil {
			return
		}
		query.Response = true
		if malformed {
			query.ID++
		}
		wire, _ = query.Pack()
		response = &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/dns-message"}}, Body: io.NopCloser(strings.NewReader(string(wire)))}
		return
	})}
	if response, err = CloudflareDNS(context.Background(), ordinaryQuery(t)); err != nil || len(response) == 0 {
		t.Fatalf("valid DoH reply: %v", err)
	}
	malformed = true
	if _, err = CloudflareDNS(context.Background(), ordinaryQuery(t)); err == nil {
		t.Fatal("accepted mismatched response")
	}
}

// TestAsyncForwarding checks upstream hops, paused completions and cancellation on reset.
func TestAsyncForwarding(t *testing.T) {
	var (
		release   chan struct{} = make(chan struct{})
		cancelled chan struct{} = make(chan struct{}, 1)
		network   *Network      = New(func(ctx context.Context, wire []byte) (response []byte, err error) {
			select {
			case <-release:
				var query dnsmessage.Message
				_ = query.Unpack(wire)
				query.Response = true
				response, err = query.Pack()
			case <-ctx.Done():
				cancelled <- struct{}{}
				err = ctx.Err()
			}
			return
		})
		frequency float64
	)
	defer network.cancel()
	apply(t, network, Input{Action: "configure", TrafficFrequency: &frequency, Servers: []Server{{ID: "dns-1", Poisoned: true, ForwardTo: "dns-2"}, {ID: "dns-2"}}})
	apply(t, network, Input{Action: "playback", Speed: 1, Playing: true})
	network.route("client-1", "dns-1", ordinaryQuery(t), nil)
	for range 3 {
		apply(t, network, Input{Action: "tick", Delta: HopDuration})
	}
	if network.Resolving != 1 || !slices.ContainsFunc(network.Events, func(event Event) bool { return event.Source == "dns-1" && event.Destination == "dns-2" }) {
		t.Fatal("upstream did not receive query before resolving")
	}
	apply(t, network, Input{Action: "playback", Speed: 1, Playing: false})
	close(release)
	for deadline := time.Now().Add(time.Second); len(network.results) == 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if len(network.results) == 0 {
		t.Fatal("resolver did not complete")
	}
	apply(t, network, Input{Action: "tick", Delta: 1})
	if network.Resolving != 1 || len(network.Packets) != 0 {
		t.Fatal("paused simulation applied a completion")
	}
	apply(t, network, Input{Action: "playback", Speed: 1, Playing: true})
	for range 4 {
		apply(t, network, Input{Action: "tick", Delta: HopDuration})
	}
	if !slices.ContainsFunc(network.Events, func(event Event) bool {
		return event.Source == "dns-2" && event.Destination == "dns-1" && event.Direction == "downstream"
	}) || network.Events[len(network.Events)-1].Destination != "client-1" {
		t.Fatal("reply skipped the forwarding server or client")
	}
	release = make(chan struct{})
	network.route("client-1", "dns-1", ordinaryQuery(t), nil)
	for range 3 {
		apply(t, network, Input{Action: "tick", Delta: HopDuration})
	}
	apply(t, network, Input{Action: "reset"})
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("reset did not cancel lookup")
	}
	if network.Resolving != 0 || len(network.Events) != 0 {
		t.Fatal("reset retained pending state")
	}
}

// TestCloudflareLive is opt-in so CI stays independent of external DNS availability.
func TestCloudflareLive(t *testing.T) {
	if os.Getenv("DNSCOMMS_LIVE_DNS") != "1" {
		t.Skip("set DNSCOMMS_LIVE_DNS=1 to exercise Cloudflare")
	}
	var (
		ctx      context.Context
		cancel   context.CancelFunc
		response []byte
		err      error
		answer   dnsmessage.Message
	)
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if response, err = CloudflareDNS(ctx, ordinaryQuery(t)); err != nil {
		t.Fatal(err)
	}
	if err = answer.Unpack(response); err != nil {
		t.Fatal(err)
	}
	if len(answer.Answers) == 0 {
		t.Fatal("Cloudflare did not return an answer")
	}
	t.Logf("Cloudflare returned %d bytes with %d answers", len(response), len(answer.Answers))
}

// TestResolverFailure keeps a failed lookup visible as SERVFAIL rather than fabricated success.
func TestResolverFailure(t *testing.T) {
	var network *Network = New()
	network.resolutionReply(resolutionResult{packet: &Packet{Destination: "dns-1", Client: "client-1", Server: "dns-1", wire: ordinaryQuery(t)}, err: errors.New("offline")})
	var (
		answer dnsmessage.Message
		err    error
	)
	if err = answer.Unpack(network.Packets[0].wire); err != nil {
		t.Fatal(err)
	}
	if answer.RCode != dnsmessage.RCodeServerFailure || !strings.Contains(network.Events[0].Outcome, "offline") {
		t.Fatal("resolver failure was hidden")
	}
}

func ordinaryQuery(t *testing.T) (wire []byte) {
	t.Helper()
	var (
		query dnsmessage.Message = dnsmessage.Message{Header: dnsmessage.Header{ID: 123, RecursionDesired: true}, Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName("example.com."), Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}}}
		err   error
	)
	if wire, err = query.Pack(); err != nil {
		t.Fatal(err)
	}
	return
}
