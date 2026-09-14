import { beforeAll, expect, test } from "bun:test";
import type { SimulationResult } from "./app/wasm";
import { buildWasm } from "./vite-wasm";

beforeAll(async () => {
    buildWasm();
    await import(/* @vite-ignore */ new URL("./public/wasm_exec.js", import.meta.url).href);
    const go = new Go();
    const { instance } = await WebAssembly.instantiate(await Bun.file(new URL("./public/main.wasm", import.meta.url)).arrayBuffer(), go.importObject);
    void go.run(instance);
});

for (const recordType of ["TXT", "A", "AAAA"]) {
    test(`${recordType} round-trips multipart UTF-8 through Go WASM`, () => {
        const message = "Hello, 世界! ".repeat(30);
        const result: SimulationResult = JSON.parse(globalThis.dnscommsSimulate(JSON.stringify({
            message: message,
            domain: "example.com",
            recordType: recordType,
            partSize: 128
        })));

        expect(result.error).toBeUndefined();
        expect(result.message).toBe(message);
        expect(result.inputBytes).toBe(new TextEncoder().encode(message).length);
        expect(result.packets.length).toBeGreaterThan(1);
        expect(result.wireBytes).toBe(result.packets.reduce((sum, packet) => sum + packet.bytes, 0));
    });
}

test("invalid requests report errors without stopping the runtime", () => {
    expect(JSON.parse(globalThis.dnscommsSimulate("{broken")).error).toBeTruthy();
    expect(JSON.parse(globalThis.dnscommsSimulate(JSON.stringify({
        message: "",
        domain: "example.com",
        recordType: "TXT",
        partSize: 256
    }))).message).toBe("");
});
