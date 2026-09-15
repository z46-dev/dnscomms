import { useState } from "react";
import type { PacketEvent } from "./wasm";
import "./capture.css";

export function Capture({
    events,
    dropped
}: {
    events: PacketEvent[];
    dropped: number;
}) {
    const [search, setSearch] = useState("");
    const [filters, setFilters] = useState<Record<string, string>>({});
    const [selected, setSelected] = useState<PacketEvent | null>(null);
    const fields = [
        "client",
        "server",
        "direction",
        "type",
        "classification"
    ] as const;
    const filtered = events.filter(
        (event) =>
            fields.every(
                (field) => !filters[field] || event[field] === filters[field]
            ) &&
            `${event.name} ${event.source} ${event.destination} ${event.outcome}`
                .toLowerCase()
                .includes(search.toLowerCase())
    );
    return (
        <>
            <div className="capture-heading">
                <h2>Packet capture</h2>
                <span className="muted">{filtered.length} hops</span>
            </div>
            <div className="capture-tools">
                <label className="search-label">
                    <span className="sr-only">Search packets</span>
                    <input
                        type="search"
                        placeholder="Search a domain or device…"
                        value={search}
                        onChange={(event) => setSearch(event.target.value)}
                    />
                </label>
                <details className="capture-filters">
                    <summary>
                        Filters
                        {Object.values(filters).filter(Boolean).length
                            ? ` (${Object.values(filters).filter(Boolean).length})`
                            : ""}
                    </summary>
                    <div className="filters">
                        {fields.map((field) => (
                            <label key={field}>
                                {field}
                                <select
                                    value={filters[field] ?? ""}
                                    onChange={(event) =>
                                        setFilters({
                                            ...filters,
                                            [field]: event.target.value
                                        })
                                    }
                                >
                                    <option value="">All</option>
                                    {[
                                        ...new Set(
                                            events.map((event) => event[field])
                                        )
                                    ]
                                        .filter(Boolean)
                                        .sort()
                                        .map((value) => (
                                            <option key={value}>{value}</option>
                                        ))}
                                </select>
                            </label>
                        ))}
                        <button
                            type="button"
                            onClick={() => {
                                setFilters({});
                                setSearch("");
                            }}
                        >
                            Clear filters
                        </button>
                    </div>
                </details>
            </div>
            <section
                className="table-scroll"
                // biome-ignore lint/a11y/noNoninteractiveTabindex: The capture region must be keyboard-scrollable.
                tabIndex={0}
                aria-label="Scrollable packet capture"
            >
                <table className="capture-table">
                    <caption className="sr-only">
                        Captured DNS traffic in arrival order
                    </caption>
                    <thead>
                        <tr>
                            <th scope="col">Time</th>
                            <th scope="col">Query</th>
                            <th scope="col">Route</th>
                        </tr>
                    </thead>
                    <tbody>
                        {filtered.map((event) => (
                            <tr
                                key={event.id}
                                className={
                                    selected?.id === event.id ? "selected" : ""
                                }
                            >
                                <td className="packet-time">
                                    {event.time.toFixed(2)}
                                    <small>#{event.id}</small>
                                </td>
                                <td>
                                    <button
                                        type="button"
                                        className="packet-query"
                                        aria-label={`Inspect event ${event.id}`}
                                        aria-pressed={selected?.id === event.id}
                                        onClick={() => setSelected(event)}
                                    >
                                        <span className="query">
                                            {event.name === "."
                                                ? "Root TXT payload"
                                                : event.name}
                                        </span>
                                    </button>
                                    <div className="packet-meta">
                                        <span>{event.type}</span>
                                        <span>{event.classification}</span>
                                    </div>
                                </td>
                                <td className="packet-route">
                                    {event.source}{" "}
                                    <span aria-hidden="true">→</span>{" "}
                                    {event.destination}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </section>
            {!filtered.length && <p className="empty">No matching packets.</p>}
            <p className="muted capture-retention">
                Latest 1,500 hops retained
                {dropped ? ` · ${dropped} older hops removed` : ""}
            </p>
            {selected && (
                <section
                    className="packet-details"
                    aria-label={`Event ${selected.id} details`}
                >
                    <div className="section-heading">
                        <h3>Packet #{selected.packetId}</h3>
                        <button type="button" onClick={() => setSelected(null)}>
                            Close details
                        </button>
                    </div>
                    <p>{selected.outcome}</p>
                    <p className="muted">
                        {selected.bytes} bytes · {selected.direction} ·{" "}
                        {selected.type}
                        {selected.trafficLabel
                            ? ` · CSV label: ${selected.trafficLabel}`
                            : ""}
                    </p>
                    {selected.part && (
                        <details open>
                            <summary>Exfil header</summary>
                            <pre>{JSON.stringify(selected.part, null, 2)}</pre>
                        </details>
                    )}
                    <details>
                        <summary>Decoded DNS fields</summary>
                        <pre>{JSON.stringify(selected.dns, null, 2)}</pre>
                    </details>
                    <details>
                        <summary>Raw hex</summary>
                        <pre>{selected.hex.match(/.{1,32}/g)?.join("\n")}</pre>
                    </details>
                </section>
            )}
        </>
    );
}
