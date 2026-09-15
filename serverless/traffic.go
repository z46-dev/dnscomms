package serverless

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

//go:embed data/dns_traffic.csv
var trafficCSV string

var traffic []trafficRow = loadTraffic()

type trafficRow struct {
	domain string
	record dnsmessage.Type
	label  string
}

// loadTraffic loads the embedded synthetic corpus once for both native and WASM builds.
func loadTraffic() (rows []trafficRow) {
	var (
		records [][]string
		err     error
	)
	if records, err = csv.NewReader(strings.NewReader(trafficCSV)).ReadAll(); err != nil {
		panic(err)
	}
	for _, record := range records[1:] {
		rows = append(rows, trafficRow{domain: record[1] + ".", record: map[string]dnsmessage.Type{"A": dnsmessage.TypeA, "AAAA": dnsmessage.TypeAAAA, "TXT": dnsmessage.TypeTXT}[record[2]], label: record[3]})
	}
	return
}

// cover samples one random background query, optionally mixing in the exfil client.
func (n *Network) cover() {
	var clients []string = []string{"client-1", "client-2"}
	if n.EvilCover {
		clients = append(clients, "exfil-client")
	}
	var (
		row   trafficRow         = traffic[n.random.IntN(len(traffic))]
		query dnsmessage.Message = dnsmessage.Message{Header: dnsmessage.Header{ID: uint16(n.ordinary + 1), RecursionDesired: true}, Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName(row.domain), Type: row.record, Class: dnsmessage.ClassINET}}}
		wire  []byte
		err   error
	)
	if wire, err = query.Pack(); err != nil {
		panic(fmt.Errorf("embedded DNS question: %w", err))
	}
	n.route(clients[n.random.IntN(len(clients))], n.Servers[n.random.IntN(len(n.Servers))].ID, wire, nil)
	n.Packets[len(n.Packets)-1].TrafficLabel = row.label
}
