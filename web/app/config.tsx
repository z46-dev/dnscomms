import type { PanelProps } from "./panels";
import type { Server } from "./wasm";

export function Config({ state, command }: PanelProps) {
    const exfilCount = state.servers.filter((server) => server.poisoned).length;
    const frequencyLabel =
        state.trafficFrequency === 0
            ? "Off"
            : `${state.trafficFrequency.toFixed(2)} queries/sec`;

    function configure(servers: Server[]) {
        const normal = new Set(
            servers
                .filter((server) => !server.poisoned)
                .map((server) => server.id)
        );
        command({
            action: "configure",
            servers: servers.map((server) => ({
                ...server,
                forwardTo:
                    server.poisoned && normal.has(server.forwardTo)
                        ? server.forwardTo
                        : ""
            }))
        });
    }
    return (
        <>
            <h2>Config</h2>
            <p className="muted">
                Changes apply between transfers. Regular DNS uses the embedded
                traffic sample and live Cloudflare answers.
            </p>
            <fieldset className="config-section" disabled={state.active}>
                <legend>Regular traffic</legend>
                <div className="config-grid">
                    <label>
                        Regular DNS frequency
                        <span className="range-value">{frequencyLabel}</span>
                        <input
                            type="range"
                            min="0"
                            max="8"
                            step="0.25"
                            value={state.trafficFrequency}
                            onChange={(event) =>
                                command({
                                    action: "configure",
                                    trafficFrequency: Number(event.target.value)
                                })
                            }
                        />
                    </label>
                    <label className="check-setting">
                        <input
                            type="checkbox"
                            checked={state.evilCover}
                            onChange={(event) =>
                                command({
                                    action: "configure",
                                    evilCover: event.target.checked
                                })
                            }
                        />
                        Allow regular DNS from the exfil client
                    </label>
                </div>
                <p className="muted">
                    1,000 synthetic queries: 900 labeled good and 100 labeled
                    bad. Labels describe the sample, not the reputation of a
                    domain.
                </p>
            </fieldset>
            <fieldset className="config-section" disabled={state.active}>
                <legend>DNS servers</legend>
                <div className="config-grid">
                    <label>
                        DNS servers (total)
                        <select
                            value={state.servers.length}
                            onChange={(event) =>
                                configure(
                                    Array.from(
                                        { length: Number(event.target.value) },
                                        (_, index) =>
                                            state.servers[index] ?? {
                                                id: `dns-${index + 1}`,
                                                poisoned: false,
                                                forwardTo: ""
                                            }
                                    )
                                )
                            }
                        >
                            {[1, 2, 3, 4, 5, 6].map((count) => (
                                <option key={count}>{count}</option>
                            ))}
                        </select>
                    </label>
                    <label>
                        Exfil servers
                        <select
                            value={exfilCount}
                            onChange={(event) => {
                                const count = Number(event.target.value);
                                const ordered = [
                                    ...state.servers.filter(
                                        (server) => server.poisoned
                                    ),
                                    ...state.servers.filter(
                                        (server) => !server.poisoned
                                    )
                                ];
                                const chosen = new Set(
                                    ordered
                                        .slice(0, count)
                                        .map((server) => server.id)
                                );
                                configure(
                                    state.servers.map((server) => ({
                                        ...server,
                                        poisoned: chosen.has(server.id)
                                    }))
                                );
                            }}
                        >
                            {[0, 1, 2, 3, 4, 5, 6]
                                .filter(
                                    (count) => count <= state.servers.length
                                )
                                .map((count) => (
                                    <option key={count}>{count}</option>
                                ))}
                        </select>
                    </label>
                </div>
                <div className="server-settings">
                    {state.servers.map((server) => (
                        <div className="server-setting" key={server.id}>
                            <label>
                                <input
                                    type="checkbox"
                                    checked={server.poisoned}
                                    onChange={() =>
                                        configure(
                                            state.servers.map((item) =>
                                                item.id === server.id
                                                    ? {
                                                          ...item,
                                                          poisoned:
                                                              !item.poisoned
                                                      }
                                                    : item
                                            )
                                        )
                                    }
                                />
                                {server.id} is an exfil server
                            </label>
                            {server.poisoned ? (
                                <label>
                                    Regular DNS on {server.id}
                                    <select
                                        value={server.forwardTo}
                                        onChange={(event) =>
                                            configure(
                                                state.servers.map((item) =>
                                                    item.id === server.id
                                                        ? {
                                                              ...item,
                                                              forwardTo:
                                                                  event.target
                                                                      .value
                                                          }
                                                        : item
                                                )
                                            )
                                        }
                                    >
                                        <option value="">
                                            Resolve directly
                                        </option>
                                        {state.servers
                                            .filter((item) => !item.poisoned)
                                            .map((item) => (
                                                <option
                                                    value={item.id}
                                                    key={item.id}
                                                >
                                                    Forward to {item.id}
                                                </option>
                                            ))}
                                    </select>
                                </label>
                            ) : (
                                <span className="muted">Resolves directly</span>
                            )}
                        </div>
                    ))}
                </div>
                <p className="muted">
                    Direct resolution uses Cloudflare DoH. Forwarding adds a hop
                    to a normal server, which resolves the query and returns its
                    answer. With no normal server, exfil servers resolve
                    directly.
                </p>
            </fieldset>
            {state.active && (
                <p className="notice">
                    Wait for the current transfer to finish before changing
                    configuration.
                </p>
            )}
        </>
    );
}
