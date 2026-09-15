import { useEffect, useState } from "react";
import type { Command, Simulation } from "./wasm";
import "./panels.css";

export type PanelProps = {
    state: Simulation;
    command: (input: Command) => void;
};

export function Exfil({ state, command }: PanelProps) {
    const [message, setMessage] = useState(
        "Hello from the inside of the firewall."
    );
    const [targets, setTargets] = useState(["dns-1"]);
    const [types, setTypes] = useState(["A"]);
    const [duration, setDuration] = useState(8);
    const [vary, setVary] = useState(false);
    const bytes = new TextEncoder().encode(message).length;
    const topology = state.servers.map((server) => server.id).join(",");
    const valid =
        bytes > 0 &&
        bytes <= 16384 &&
        targets.length > 0 &&
        types.length > 0 &&
        duration >= 1 &&
        duration <= 120;
    const maxPayload = types.includes("TXT") ? 65232 : 163;
    const minPayload = types.some((type) => type !== "TXT") ? 163 : 65232;
    useEffect(() => {
        setTargets((current) =>
            current.filter((target) => topology.split(",").includes(target))
        );
    }, [topology]);

    return (
        <>
            <section className="send-section" aria-label="Exfil client">
                <div className="section-heading">
                    <h2>Send a message</h2>
                    <span className="eyebrow">CLIENT → ORCHESTRATOR</span>
                </div>
                <form
                    onSubmit={(event) => {
                        event.preventDefault();
                        command({
                            action: "send",
                            message,
                            targets,
                            types,
                            duration,
                            vary
                        });
                    }}
                >
                    <label htmlFor="message">Message</label>
                    <textarea
                        id="message"
                        value={message}
                        rows={4}
                        onChange={(event) => setMessage(event.target.value)}
                        aria-describedby="message-size"
                    />
                    <p
                        id="message-size"
                        className={bytes > 16384 ? "error" : "muted"}
                    >
                        {bytes.toLocaleString()} / 16,384 bytes ·{" "}
                        {types.length
                            ? `${Math.ceil(bytes / maxPayload)}–${vary ? bytes : Math.ceil(bytes / minPayload)} estimated requests`
                            : "Select a record type"}
                    </p>
                    <div className="send-options">
                        <fieldset disabled={state.active}>
                            <legend>Targets</legend>
                            <div className="choices">
                                {state.servers.map((server) => (
                                    <label key={server.id}>
                                        <input
                                            type="checkbox"
                                            checked={targets.includes(
                                                server.id
                                            )}
                                            onChange={() =>
                                                setTargets(
                                                    toggle(targets, server.id)
                                                )
                                            }
                                        />
                                        {server.id}
                                        <span className="muted">
                                            {server.poisoned
                                                ? "exfil"
                                                : "normal"}
                                        </span>
                                    </label>
                                ))}
                            </div>
                        </fieldset>
                        <fieldset disabled={state.active}>
                            <legend>Record types</legend>
                            <div className="choices">
                                {["A", "AAAA", "TXT"].map((type) => (
                                    <label key={type}>
                                        <input
                                            type="checkbox"
                                            checked={types.includes(type)}
                                            onChange={() =>
                                                setTypes(toggle(types, type))
                                            }
                                        />
                                        {type}
                                    </label>
                                ))}
                            </div>
                        </fieldset>
                    </div>
                    <div className="send-footer">
                        <label>
                            Duration (seconds)
                            <input
                                type="number"
                                min={1}
                                max={120}
                                required
                                value={duration}
                                onChange={(event) =>
                                    setDuration(Number(event.target.value))
                                }
                            />
                        </label>
                        <label>
                            <input
                                type="checkbox"
                                checked={vary}
                                onChange={(event) =>
                                    setVary(event.target.checked)
                                }
                            />
                            Vary payload lengths
                        </label>
                        <button
                            className="primary"
                            type="submit"
                            disabled={!valid || state.active}
                        >
                            {state.active
                                ? "Transfer in progress"
                                : "Send transfer"}
                        </button>
                    </div>
                    {!valid && (
                        <p className="error">
                            Enter 1–16,384 bytes, choose targets and types, and
                            a duration of 1–120 seconds.
                        </p>
                    )}
                    {targets.length > 0 &&
                        !state.servers.some(
                            (server) =>
                                server.poisoned && targets.includes(server.id)
                        ) && (
                            <p className="notice">
                                Normal targets won’t forward your message to the
                                orchestrator.
                            </p>
                        )}
                </form>
            </section>
            <section className="inbox" aria-label="Orchestrator inbox">
                <div className="section-heading">
                    <h2>Orchestrator inbox</h2>
                    <span className="muted">
                        {state.transfers.length}{" "}
                        {state.transfers.length === 1
                            ? "transfer"
                            : "transfers"}
                    </span>
                </div>
                {!state.transfers.length && (
                    <p className="empty">
                        Incoming parts and recovered messages will appear here.
                    </p>
                )}
                {[...state.transfers].reverse().map((transfer) => (
                    <article className="transfer" key={transfer.id}>
                        <div className="section-heading">
                            <h3>{transfer.id}</h3>
                            <span className={`badge ${transfer.status}`}>
                                {transfer.status}
                            </span>
                        </div>
                        <progress
                            value={transfer.received}
                            max={transfer.expected}
                            aria-label={`Received parts for ${transfer.id}`}
                        />
                        <p className="muted">
                            {transfer.sent} sent · {transfer.received} /{" "}
                            {transfer.expected} received · {transfer.inputBytes}{" "}
                            bytes
                        </p>
                        {transfer.status === "complete" && (
                            <pre className="recovered">{transfer.message}</pre>
                        )}
                        <details>
                            <summary>Parts & delivery</summary>
                            <p>
                                From{" "}
                                {transfer.sources.join(", ") ||
                                    "no servers yet"}
                            </p>
                            <p className="sequence">
                                Missing sequences:{" "}
                                {transfer.missing.join(", ") || "none"}
                            </p>
                            <div className="queue">
                                {transfer.requests.map((request) => (
                                    <p key={request.sequence}>
                                        #{request.sequence} · {request.type} →{" "}
                                        {request.target} · {request.bytes} bytes
                                        · {request.status}
                                    </p>
                                ))}
                            </div>
                        </details>
                    </article>
                ))}
            </section>
        </>
    );
}

function toggle(values: string[], value: string) {
    return values.includes(value)
        ? values.filter((item) => item !== value)
        : [...values, value];
}
