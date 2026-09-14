export interface SimulationInput {
    message: string;
    domain: string;
    recordType: string;
    partSize: number;
}

export interface SimulationResult {
    packets: { bytes: number; hex: string }[];
    message: string;
    inputBytes: number;
    wireBytes: number;
    error?: string;
}

export type Simulator = (input: SimulationInput) => SimulationResult;

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

        return (input: SimulationInput) => {
            const result: SimulationResult = JSON.parse(globalThis.dnscommsSimulate(JSON.stringify(input)));

            if (result.error) {
                throw new Error(result.error);
            }

            return result;
        };
    })();

    return runtime;
}
