# Web network activity plan

## Goal

Build a browser-run DNS network activity that shows an exfiltrating client alongside ordinary DNS traffic. The activity should make each encoded request, firewall crossing, DNS server decision, and transfer to the orchestrator visible. Run the network in memory; the “real DNS” cover traffic should use ordinary DNS questions and plausible, deterministic answers from the simulated servers, rather than contacting live resolvers.

## Screen layout

Use a responsive 70/30 split: a 70% control and inspection column on the left and a 30% network view on the right. On narrow screens, stack the network view above the controls and keep both usable without horizontal scrolling. A compact header shows the activity title and a shared simulation status. The right panel remains visible as the left tabs change.

The left panel has three keyboard-accessible tabs:

1. **Exfiltrating client.** A message editor, server targets, record-type choices (A, AAAA, TXT), duration, payload-length variation, and a send control. Show validation, input byte count, estimated request count, progress, and the client's queued/sent requests. Sending creates a new transfer with a unique exfil ID; it does not erase existing packet history. A reset control clears the whole simulation.
2. **Orchestrator.** Show transfer groups by exfil ID, received/expected parts, missing sequence numbers, source DNS servers, and assembly status. Once all parts arrive, display the recovered message and its byte count. Partial and failed transfers remain inspectable. Configuration includes which DNS servers are poisoned and how many DNS servers exist; changing topology should be disabled during an active transfer or applied only after it completes.
3. **Packet capture.** A chronological table of every request and response crossing the firewall and every poisoned-server-to-orchestrator transfer. Include time/order, source, destination, direction, record type, query name or summary, wire bytes, and classification (ordinary DNS, exfil part, cover response, forwarded part). Filters for client, server, direction, type, and classification should operate on the same event log. Selecting a row opens packet details: decoded DNS fields, exfil header when present, and raw hex. Keep the table accessible to keyboard and screen readers.

The right panel contains a canvas network map rendered with `github.com/z46-dev/wasmdraw/ctx2d` from Go/WASM. React owns the surrounding controls and text, while canvas renders nodes, links, packets in flight, and status highlights. Provide a text summary for events represented by canvas animation so the simulation is still understandable without motion or canvas.

## Network and behavior

Place three clients on the inside of a visible firewall boundary: one exfiltrating client and two ordinary clients. Place a configurable number of DNS servers and one orchestrator outside. Give each DNS server a stable label and a visible normal/poisoned state. Draw client-to-server links through the firewall and poisoned-server-to-orchestrator links outside it.

The two ordinary clients automatically issue genuine DNS-shaped A, AAAA, or TXT queries at a controlled interval, with normal replies. These requests are not directly editable by the user, but their timing and answers should be reproducible from a fixed simulation seed. Both normal and poisoned DNS servers answer ordinary queries, so poisoned servers visibly maintain cover behavior.

For an exfil send, use `codec.EncodeExfilRequest` to split the message and `ExfilRequest.Encode` to create wire packets. Route each request to its selected target DNS server. A poisoned server tries `codec.DecodeExfilRequest`: a valid exfil part is accepted, logged, and forwarded to the orchestrator; a failed parse falls back to ordinary DNS handling. A normal server always handles it as an ordinary DNS question. Show the response or rejection in the capture and animation. The orchestrator collects parts by exfil ID, validates sequence/total, calls `codec.RebuildExfilData` when complete, and publishes the result. Make lost or misrouted parts visible as an incomplete transfer rather than silently recovering data.

Every hop emits a structured event with an ID, timestamp or simulation tick, source, destination, DNS metadata, optional exfil metadata, outcome, and packet bytes. The event log is the single source for capture rows, node highlights, counters, and playback. Animation should distinguish ordinary DNS, exfil DNS, responses, and orchestrator forwarding by color and direction, with a legend. Add play/pause, speed, and reduced-motion behavior; pausing freezes motion but preserves capture and orchestrator state. Use a bounded history or explicit reset so background ordinary traffic does not grow memory indefinitely.

## Implementation path

1. Define Go simulation types and a browser-facing JSON interface for topology configuration, send, tick/playback, events, and transfer status. Keep encoding, routing, DNS handling, and assembly in Go so the browser uses the real codec. Validate server count and ensure at least one poisoned target can be selected for a successful demo.
2. Replace the current web WASM example interface: `web/src/main.go` still references older `ExfilPart`/`EncodeExfilDNS` APIs. Expose the new simulation calls and TypeScript types through `web/app/wasm.ts`; keep the Go/WASM runtime loaded once.
3. Build the three React tabs with shared simulation state and event selection. Keep controls and packet details in semantic HTML; do not put readable data solely in canvas.
4. Add the ctx2d canvas renderer with resize-aware coordinates, a fixed topology layout, and event-driven packet animation. Keep renderer state separate from React form state.
5. Validate codec round trips for A, AAAA, and TXT, mixed ordinary/exfil traffic, poisoned and normal routing, incomplete and complete assembly, and capture ordering. Run Go tests, web type checks/build, and the repository's workflow checks. Terminate any development server used for visual review.

## Acceptance criteria

- A message sent to poisoned DNS servers can be reconstructed by the orchestrator, with every packet visible in capture and on the map.
- A request sent to a normal server, or an incomplete transfer, remains visible and does not appear as recovered data.
- Both ordinary clients continually make DNS requests and receive valid simulated replies from normal and poisoned servers.
- Server count and poisoned identity can be changed between runs; the map and controls stay in sync.
- Tabs, controls, packet details, and status remain usable with keyboard input, on narrow screens, and with reduced motion.
- No excessive lines of code or unmaintainable hacks; the simulation is a clean, testable Go/WASM implementation with a React interface. We should strive for a clean minimalistic UI that does not fall into common AI-generated design pitfalls. The simulation should be understandable and usable by someone with a basic understanding of DNS and network traffic, without requiring deep technical knowledge or many tooltips littering the design.