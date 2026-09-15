export {};

declare global {
    // Supplied by the runtime copied from the Go installation during the build.
    class Go {
        importObject: WebAssembly.Imports;
        run(instance: WebAssembly.Instance): Promise<void>;
    }

    // Registered synchronously when the Go simulator starts.
    function dnscommsCanvas(reducedMotion: boolean): string;
    function dnscommsSimulate(input: string): string;
}
