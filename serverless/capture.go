package serverless

import (
	"encoding/hex"
	"net"

	"golang.org/x/net/dns/dnsmessage"
)

// emit records a completed hop and the receiving device’s decision.
func (n *Network) emit(packet *Packet, outcome string) {
	var (
		message dnsmessage.Message
		err     error
	)
	if err = message.Unpack(packet.wire); err != nil || len(message.Questions) != 1 {
		return
	}
	n.nextID++
	n.Events = append(n.Events, Event{TrafficLabel: packet.TrafficLabel, ID: n.nextID, PacketID: packet.ID, Time: n.Time, Source: packet.Source, Destination: packet.Destination, Client: packet.Client, Server: packet.Server, Direction: packet.Direction, Type: recordName(message.Questions[0].Type), Name: message.Questions[0].Name.String(), Classification: packet.Classification, Outcome: outcome, Bytes: len(packet.wire), Hex: hex.EncodeToString(packet.wire), DNS: decodedFields(message), Part: packet.part})
	if len(n.Events) > HistoryLimit {
		copy(n.Events, n.Events[len(n.Events)-HistoryLimit:])
		n.Events = n.Events[:HistoryLimit]
		n.Dropped++
	}
}

// decodedFields keeps packet details readable without exposing DNS name backing buffers.
func decodedFields(message dnsmessage.Message) (fields map[string]any) {
	var resources []map[string]any = []map[string]any{}
	for _, section := range []struct {
		name    string
		records []dnsmessage.Resource
	}{{"answer", message.Answers}, {"authority", message.Authorities}, {"additional", message.Additionals}} {
		for _, resource := range section.records {
			resources = append(resources, map[string]any{"section": section.name, "name": resource.Header.Name.String(), "type": recordName(resource.Header.Type), "class": uint16(resource.Header.Class), "ttl": resource.Header.TTL, "data": resourceData(resource.Body)})
		}
	}
	fields = map[string]any{
		"header":          message.Header,
		"question":        map[string]any{"name": message.Questions[0].Name.String(), "type": recordName(message.Questions[0].Type), "class": uint16(message.Questions[0].Class)},
		"answerCount":     len(message.Answers),
		"additionalCount": len(message.Additionals),
		"resources":       resources,
	}
	return
}

// resourceData presents addresses and names without DNS name backing buffers.
func resourceData(body dnsmessage.ResourceBody) (data any) {
	switch value := body.(type) {
	case *dnsmessage.AResource:
		data = net.IP(value.A[:]).String()
	case *dnsmessage.AAAAResource:
		data = net.IP(value.AAAA[:]).String()
	case *dnsmessage.CNAMEResource:
		data = value.CNAME.String()
	case *dnsmessage.NSResource:
		data = value.NS.String()
	case *dnsmessage.PTRResource:
		data = value.PTR.String()
	case *dnsmessage.TXTResource:
		data = value.TXT
	case *dnsmessage.MXResource:
		data = map[string]any{"preference": value.Pref, "exchange": value.MX.String()}
	case *dnsmessage.SOAResource:
		data = map[string]any{"primary": value.NS.String(), "mailbox": value.MBox.String(), "serial": value.Serial, "refresh": value.Refresh, "retry": value.Retry, "expire": value.Expire, "minimumTTL": value.MinTTL}
	default:
		data = body
	}
	return
}
