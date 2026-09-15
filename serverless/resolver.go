package serverless

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

type (
	Resolver         func(context.Context, []byte) ([]byte, error)
	resolutionResult struct {
		packet *Packet
		wire   []byte
		err    error
	}
)

// CloudflareDNS fetches and validates a real DNS wire response using Go's HTTP transport.
func CloudflareDNS(ctx context.Context, wire []byte) (response []byte, err error) {
	var (
		request       *http.Request
		result        *http.Response
		query, answer dnsmessage.Message
	)
	if err = query.Unpack(wire); err != nil {
		return
	}
	if len(query.Questions) != 1 {
		return nil, errors.New("expected one DNS question")
	}
	if request, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://cloudflare-dns.com/dns-query?dns="+base64.RawURLEncoding.EncodeToString(wire), nil); err != nil {
		return
	}
	request.Header.Set("Accept", "application/dns-message")
	if result, err = http.DefaultClient.Do(request); err != nil {
		return
	}
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK || !strings.HasPrefix(result.Header.Get("Content-Type"), "application/dns-message") {
		return nil, fmt.Errorf("Cloudflare returned HTTP %d (%s)", result.StatusCode, result.Header.Get("Content-Type"))
	}
	if response, err = io.ReadAll(io.LimitReader(result.Body, 65536)); err != nil {
		return
	}
	if len(response) > 65535 {
		return nil, errors.New("DNS response exceeds wire size limit")
	}
	if err = answer.Unpack(response); err != nil {
		return
	}
	if !answer.Response || answer.ID != query.ID || len(answer.Questions) != 1 || answer.Questions[0] != query.Questions[0] {
		return nil, errors.New("Cloudflare response does not match the DNS question")
	}
	return
}

// resolve starts bounded asynchronous work, keeping HTTP waits outside the game loop.
func (n *Network) resolve(packet *Packet) {
	if n.Resolving >= cap(n.results) {
		n.resolutionReply(resolutionResult{packet: packet, err: errors.New("resolver busy")})
		return
	}
	n.Resolving++
	var (
		results  chan resolutionResult = n.results
		resolver Resolver              = n.resolver
		ctx      context.Context
		lifetime context.Context = n.context
		cancel   context.CancelFunc
	)
	ctx, cancel = context.WithTimeout(n.context, 5*time.Second)
	go func() {
		defer cancel()
		var result resolutionResult = resolutionResult{packet: packet}
		result.wire, result.err = resolver(ctx, packet.wire)
		select {
		case results <- result:
		case <-lifetime.Done():
		}
	}()
}

// updateResolutions applies finished HTTP results only on running simulation ticks.
func (n *Network) updateResolutions() {
	for {
		select {
		case result := <-n.results:
			n.Resolving--
			n.resolutionReply(result)
		default:
			return
		}
	}
}

// resolutionReply converts transport failures into visible DNS SERVFAIL replies.
func (n *Network) resolutionReply(result resolutionResult) {
	if !slices.ContainsFunc(n.Servers, func(server Server) bool { return server.ID == result.packet.Destination }) {
		n.emit(result.packet, "dropped: resolving server removed")
		return
	}
	if result.err != nil {
		var query dnsmessage.Message
		_ = query.Unpack(result.packet.wire)
		query.Response = true
		query.RCode = dnsmessage.RCodeServerFailure
		query.Additionals = nil
		result.wire, _ = query.Pack()
		n.emit(result.packet, "resolution failed: "+result.err.Error())
	}
	n.reply(result.packet, result.wire)
}

// reply returns a DNS answer through its original forwarding server and firewall.
func (n *Network) reply(request *Packet, wire []byte) {
	var response *Packet = &Packet{Source: request.Destination, Destination: "firewall", Client: request.Client, Server: request.Server, Direction: "inbound", Classification: "cover response", wire: wire, sequence: request.sequence, transfer: request.transfer, TrafficLabel: request.TrafficLabel}
	if request.via != "" {
		response.Destination = request.via
		response.Direction = "downstream"
	}
	n.launch(response)
}
