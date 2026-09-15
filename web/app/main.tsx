import { useCallback, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { Capture } from "./capture";
import { Config } from "./config";
import { Exfil } from "./panels";
import {
    type Command,
    loadSimulator,
    type Simulation,
    type Simulator
} from "./wasm";
import "./style.css";

const tabs = ["DNS Exfil", "Packet capture", "Config"];

function App() {
    const [simulator, setSimulator] = useState<Simulator | null>(null);
    const [state, setState] = useState<Simulation | null>(null);
    const [error, setError] = useState("");
    const [tab, setTab] = useState(0);
    const [generation, setGeneration] = useState(0);
    const command = useCallback(
        (input: Command) => {
            if (!simulator) return;
            try {
                setState(simulator(input));
                if (input.action === "reset")
                    setGeneration((value) => value + 1);
                setError("");
            } catch (error) {
                setError(
                    error instanceof Error ? error.message : String(error)
                );
            }
        },
        [simulator]
    );

    useEffect(() => {
        void loadSimulator()
            .then((run) => {
                setSimulator(() => run);
                setState(run({ action: "state" }));
            })
            .catch((error: unknown) => setError(String(error)));
    }, []);

    useEffect(() => {
        if (!simulator) return;
        const preference = matchMedia("(prefers-reduced-motion: reduce)");
        const attach = () => {
            const error = globalThis.dnscommsCanvas(preference.matches);
            if (error) setError(error);
        };
        attach();
        preference.addEventListener("change", attach);
        const timer = setInterval(() => {
            setState(simulator({ action: "state" }));
        }, 100);
        return () => {
            clearInterval(timer);
            preference.removeEventListener("change", attach);
        };
    }, [simulator]);

    const latest = state?.events.at(-1);
    return (
        <main>
            <header>
                <div>
                    <h1>DNS network activity</h1>
                    <p>Follow a message through a simulated network.</p>
                </div>
                <span role="status">
                    {state
                        ? `${state.playing ? "Running" : "Paused"} · ${state.time.toFixed(1)}s`
                        : "Loading…"}
                </span>
            </header>
            {error && (
                <p role="alert" className="error">
                    {error}
                </p>
            )}
            <div className="workspace">
                <section
                    className="inspection"
                    aria-label="Controls and inspection"
                >
                    <div
                        role="tablist"
                        aria-label="Inspection views"
                        className="tabs"
                    >
                        {tabs.map((name, index) => (
                            <button
                                key={name}
                                type="button"
                                role="tab"
                                id={`tab-${index}`}
                                aria-controls={`panel-${index}`}
                                aria-selected={tab === index}
                                tabIndex={tab === index ? 0 : -1}
                                onClick={() => setTab(index)}
                                onKeyDown={(event) => {
                                    let next = index;
                                    if (event.key === "ArrowRight")
                                        next = (index + 1) % tabs.length;
                                    else if (event.key === "ArrowLeft")
                                        next =
                                            (index + tabs.length - 1) %
                                            tabs.length;
                                    else if (event.key === "Home") next = 0;
                                    else if (event.key === "End")
                                        next = tabs.length - 1;
                                    else return;
                                    event.preventDefault();
                                    setTab(next);
                                    document
                                        .getElementById(`tab-${next}`)
                                        ?.focus();
                                }}
                            >
                                {name}
                            </button>
                        ))}
                    </div>
                    {state &&
                        tabs.map((name, index) => (
                            <section
                                key={name}
                                role="tabpanel"
                                id={`panel-${index}`}
                                aria-labelledby={`tab-${index}`}
                                hidden={tab !== index}
                            >
                                {index === 0 && (
                                    <Exfil state={state} command={command} />
                                )}
                                {index === 1 && (
                                    <Capture
                                        key={generation}
                                        events={state.events}
                                        dropped={state.dropped}
                                    />
                                )}
                                {index === 2 && (
                                    <Config state={state} command={command} />
                                )}
                            </section>
                        ))}
                </section>
                <aside aria-label="Live network" className="network">
                    <h2>Network</h2>
                    <div className="toolbar">
                        <button
                            type="button"
                            disabled={!state}
                            onClick={() =>
                                command({
                                    action: "playback",
                                    playing: !state?.playing,
                                    speed: state?.speed
                                })
                            }
                        >
                            {state?.playing ? "Pause" : "Play"}
                        </button>
                        <label>
                            Speed{" "}
                            <select
                                value={state?.speed ?? 1}
                                onChange={(event) =>
                                    command({
                                        action: "playback",
                                        playing: state?.playing,
                                        speed: Number(event.target.value)
                                    })
                                }
                            >
                                {[0.5, 1, 2, 4].map((speed) => (
                                    <option key={speed} value={speed}>
                                        {speed}×
                                    </option>
                                ))}
                            </select>
                        </label>
                        <button
                            type="button"
                            onClick={() => command({ action: "reset" })}
                        >
                            Reset
                        </button>
                    </div>
                    <canvas
                        id="network-map"
                        aria-label="Three inside clients, a firewall, DNS servers and an outside orchestrator. Traffic travels through the firewall device. Packet activity is listed below and in Packet capture."
                    />
                    <ul className="legend">
                        <li className="ordinary">Regular DNS ↓</li>
                        <li className="exfil">Exfil DNS ↓</li>
                        <li className="response">Response ↑</li>
                        <li className="forward">Forwarded part ↓</li>
                    </ul>
                    <p className="event-summary">
                        {latest
                            ? `#${latest.id} ${latest.source} → ${latest.destination}: ${latest.classification}. ${latest.outcome}.`
                            : "Waiting for the first DNS request."}
                    </p>
                    <p>
                        {(state?.events.length ?? 0) + (state?.dropped ?? 0)}{" "}
                        captured hops · {state?.packets.length ?? 0} in flight ·{" "}
                        {state?.resolving ?? 0} resolving ·{" "}
                        {state?.transfers.filter(
                            (transfer) => transfer.status === "complete"
                        ).length ?? 0}{" "}
                        recovered transfers
                    </p>
                    <p className="muted">
                        Regular queries use Cloudflare DNS over HTTPS. Your
                        exfil message stays in the browser. Reset clears the
                        activity.
                    </p>
                </aside>
            </div>
        </main>
    );
}

const root = document.getElementById("root");
if (!root) throw new Error("Missing application root.");
createRoot(root).render(<App />);
