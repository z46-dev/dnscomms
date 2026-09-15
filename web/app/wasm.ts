export interface Server {
    id: string;
    poisoned: boolean;
    forwardTo: string;
}
export interface Part {
    id: string;
    sequence: number;
    total: number;
    bodyBytes: number;
    flags: number;
}
export interface PacketEvent {
    trafficLabel?: string;
    packetId: number;
    client: string;
    server: string;
    id: number;
    time: number;
    source: string;
    destination: string;
    direction: string;
    type: string;
    name: string;
    classification: string;
    outcome: string;
    bytes: number;
    hex: string;
    dns: unknown;
    part?: Part;
}
export interface Transfer {
    id: string;
    expected: number;
    received: number;
    sent: number;
    missing: number[];
    sources: string[];
    status: string;
    message: string;
    inputBytes: number;
    requests: {
        sequence: number;
        target: string;
        type: string;
        due: number;
        status: string;
        bytes: number;
    }[];
}
export interface Simulation {
    trafficFrequency: number;
    evilCover: boolean;
    liveDNS: boolean;
    resolving: number;
    packets: {
        id: number;
        source: string;
        destination: string;
        client: string;
        server: string;
        direction: string;
        classification: string;
        progress: number;
    }[];
    time: number;
    playing: boolean;
    speed: number;
    servers: Server[];
    events: PacketEvent[];
    transfers: Transfer[];
    active: boolean;
    dropped: number;
    error?: string;
}
export interface Command {
    trafficFrequency?: number;
    evilCover?: boolean;
    action: "state" | "send" | "configure" | "tick" | "playback" | "reset";
    message?: string;
    targets?: string[];
    types?: string[];
    duration?: number;
    vary?: boolean;
    servers?: Server[];
    delta?: number;
    playing?: boolean;
    speed?: number;
}
export type Simulator = (input: Command) => Simulation;

let runtime: Promise<Simulator> | undefined;

export function loadSimulator(): Promise<Simulator> {
    runtime ??= (async () => {
        const go = new Go();
        const response = await fetch(`${import.meta.env.BASE_URL}main.wasm`);
        if (!response.ok) {
            throw new Error(`Could not load simulator (${response.status}).`);
        }

        const { instance } = await WebAssembly.instantiate(
            await response.arrayBuffer(),
            go.importObject
        );

        // Go registers the bridge before yielding; its main function stays alive.
        void go.run(instance);
        if (!globalThis.dnscommsSimulate) {
            throw new Error("Simulator failed to start.");
        }

        return (input: Command) => {
            const result: Simulation = JSON.parse(
                globalThis.dnscommsSimulate(JSON.stringify(input))
            );

            if (result.error) {
                throw new Error(result.error);
            }

            return result;
        };
    })();

    return runtime;
}
