export {};

declare global {
    // Supplied by the runtime copied from the Go installation during the build.
    class Go {
        importObject: WebAssembly.Imports;
        run(instance: WebAssembly.Instance): Promise<void>;
    }

    // Registered synchronously when the Go simulator starts.
    function dnscommsSimulate(input: string): string;
}
