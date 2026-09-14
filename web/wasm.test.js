import { beforeAll, expect, test } from "bun:test";
import { buildWasm } from "./vite-wasm.js";

beforeAll(async () => {
    buildWasm();
    await import("./public/wasm_exec.js");
    const go = new globalThis.Go();
    const { instance } = await WebAssembly.instantiate(
        await Bun.file(
            new URL("./public/main.wasm", import.meta.url)
        ).arrayBuffer(),
        go.importObject
    );
    void go.run(instance);
});

for (const recordType of ["TXT", "A", "AAAA"]) {
    test(`${recordType} round-trips multipart UTF-8 through Go WASM`, () => {
        const message = "Hello, 世界! ".repeat(30);
        const result = JSON.parse(
            globalThis.dnscommsSimulate(
                JSON.stringify({
                    message,
                    domain: "example.com",
                    recordType,
                    partSize: 128
                })
            )
        );
        expect(result.error).toBeUndefined();
        expect(result.message).toBe(message);
        expect(result.inputBytes).toBe(
            new TextEncoder().encode(message).length
        );
        expect(result.packets.length).toBeGreaterThan(1);
        expect(result.wireBytes).toBe(
            result.packets.reduce((sum, packet) => sum + packet.bytes, 0)
        );
    });
}

test("invalid requests report errors without stopping the runtime", () => {
    expect(
        JSON.parse(globalThis.dnscommsSimulate("{broken")).error
    ).toBeTruthy();
    expect(
        JSON.parse(
            globalThis.dnscommsSimulate(
                JSON.stringify({
                    message: "",
                    domain: "example.com",
                    recordType: "TXT",
                    partSize: 256
                })
            )
        ).message
    ).toBe("");
});
