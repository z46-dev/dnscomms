let runtime;

export function loadSimulator() {
    runtime ??= (async () => {
        const go = new globalThis.Go();
        const response = await fetch(`${import.meta.env.BASE_URL}main.wasm`);
        if (!response.ok)
            throw new Error(`Could not load simulator (${response.status}).`);
        const { instance } = await WebAssembly.instantiate(
            await response.arrayBuffer(),
            go.importObject
        );
        // Go registers the bridge before yielding; its main function stays alive.
        void go.run(instance);
        if (!globalThis.dnscommsSimulate)
            throw new Error("Simulator failed to start.");
        return (input) => {
            const result = JSON.parse(
                globalThis.dnscommsSimulate(JSON.stringify(input))
            );
            if (result.error) throw new Error(result.error);
            return result;
        };
    })();
    return runtime;
}
