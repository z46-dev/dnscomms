import { beforeAll, expect, test } from "bun:test";
import type { Command, Simulation } from "./app/wasm";
import { buildWasm } from "./vite-wasm";

beforeAll(async () => {
    buildWasm();
    await import(
        /* @vite-ignore */ new URL("./public/wasm_exec.js", import.meta.url)
            .href
    );
    const go = new Go();
    const { instance } = await WebAssembly.instantiate(
        await Bun.file(
            new URL("./public/main.wasm", import.meta.url)
        ).arrayBuffer(),
        go.importObject
    );
    void go.run(instance);
});

function run(command: Command): Simulation {
    return JSON.parse(globalThis.dnscommsSimulate(JSON.stringify(command)));
}

for (const type of ["TXT", "A", "AAAA"]) {
    test(`${type} transfers UTF-8 through the real Go WASM network`, () => {
        const reset = run({ action: "reset" });
        expect(reset.playing).toBe(false);
        expect(reset.trafficFrequency).toBe(0.5);
        expect(reset.servers[2]?.forwardTo).toBe("dns-2");
        run({ action: "configure", trafficFrequency: 0 });
        run({ action: "playback", playing: true, speed: 1 });
        const message = "Hello, 世界! ".repeat(30);
        const initial = run({
            action: "send",
            message,
            types: [type],
            targets: ["dns-1"],
            duration: 2,
            vary: true
        });
        expect(initial.error).toBeUndefined();
        expect(initial.transfers[0]?.status).toBe("sending");
        run({ action: "tick", delta: 1 });
        for (let tick = 0; tick < 4; tick++) run({ action: "tick", delta: 1 });
        const result = run({ action: "tick", delta: 1 });
        expect(result.transfers[0]?.message).toBe(message);
        expect(result.transfers[0]?.status).toBe("complete");
        expect(result.transfers[0]?.inputBytes).toBe(
            new TextEncoder().encode(message).length
        );
        expect(
            result.events.filter(
                (event) => event.classification === "forwarded part"
            ).length
        ).toBe(result.transfers[0]?.expected ?? 0);
    });
}

test("normal routing, paused playback and invalid commands preserve runtime integrity", () => {
    run({ action: "reset" });
    run({ action: "configure", trafficFrequency: 0 });
    run({
        action: "send",
        message: "Lost message",
        targets: ["dns-2"],
        types: ["A"],
        duration: 1
    });
    expect(run({ action: "tick", delta: 1 }).time).toBe(0);
    run({ action: "playback", playing: true, speed: 1 });
    for (let tick = 0; tick < 4; tick++) run({ action: "tick", delta: 1 });
    const result = run({ action: "tick", delta: 1 });
    expect(result.transfers[0]?.status).toBe("incomplete");
    expect(result.transfers[0]?.message).toBe("");
    expect(
        result.events.some((event) => event.outcome.includes("NXDOMAIN"))
    ).toBe(true);
    expect(
        JSON.parse(globalThis.dnscommsSimulate("{broken")).error
    ).toBeTruthy();
    expect(
        run({
            action: "send",
            message: "",
            targets: ["dns-1"],
            types: ["TXT"],
            duration: 1
        }).error
    ).toBeTruthy();
    expect(run({ action: "state" }).transfers).toHaveLength(1);
});
