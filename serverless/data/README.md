# Synthetic DNS traffic

`dns_traffic.csv` contains 1,000 synthetic queries with columns `id,domain,qtype,label`: 900 labeled `good` and 100 labeled `bad`. IDs run consecutively from 1 through 1,000. Query types are A (690), AAAA (245), and TXT (65).

Normal traffic samples public website, package registry, infrastructure, and organization names. Repeated names represent recurring lookups. Labels describe the simulated traffic and make no claim about the reputation of the domains or the presence of a particular DNS record. Names and records were not checked against live DNS when creating this fixture.

The 100 bad rows comprise 20 examples each of Base32-encoded synthetic payloads, random DGA-like names, deep subdomains, TXT command-like requests, and recurring beacon names. These names use the reserved example.com, example.net, and example.org domains. Payloads contain fabricated sample text. TXT requests also appear in good traffic because query type alone does not establish maliciousness.

Rows are shuffled deterministically. CSV ordering has no timing meaning; beacon rows repeat four host names five times each, and the simulator determines the timing of playback. The file is embedded in Go by `serverless/traffic.go`; the simulator samples all rows and retains each label in packet details. Regular queries obtain live Cloudflare DNS-over-HTTPS answers at runtime. The corpus can also be filtered by `label` when a consumer needs only synthetic good traffic.
