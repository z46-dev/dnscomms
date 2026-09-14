import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { loadSimulator } from "./wasm.js";
import "./style.css";

function App() {
    const [simulate, setSimulate] = useState(null);
    const [error, setError] = useState("");
    const [result, setResult] = useState(null);

    useEffect(() => {
        loadSimulator()
            .then((simulateMessage) => setSimulate(() => simulateMessage))
            .catch((error) => setError(error.message));
    }, []);

    function handleSubmit(event) {
        event.preventDefault();
        setError("");
        setResult(null);
        const form = new FormData(event.currentTarget);
        try {
            setResult(
                simulate({
                    message: form.get("message"),
                    domain: form.get("domain"),
                    recordType: form.get("recordType"),
                    partSize: Number(form.get("partSize"))
                })
            );
        } catch (error) {
            setError(error.message);
        }
    }

    return (
        <main className="mx-auto max-w-3xl px-5 py-10 sm:py-16">
            <header className="mb-10 flex items-baseline justify-between border-b border-stone-300 pb-4">
                <h1 className="text-xl font-semibold tracking-tight">
                    dnscomms
                </h1>
                <span className="text-sm text-stone-500">Local simulation</span>
            </header>
            <form onSubmit={handleSubmit} className="space-y-5">
                <label className="field-label block">
                    Message
                    <textarea
                        name="message"
                        defaultValue="Hello, World!"
                        rows={4}
                        maxLength={16384}
                        className="form-control focus-ring mt-2 block w-full resize-y font-mono"
                    />
                </label>
                <div className="grid gap-4 sm:grid-cols-[2fr_1fr_1fr]">
                    <label className="field-label">
                        Domain
                        <input
                            name="domain"
                            defaultValue="example.com"
                            required
                            className="form-control focus-ring mt-2 block w-full"
                        />
                    </label>
                    <label className="field-label">
                        Record
                        <select
                            name="recordType"
                            className="form-control focus-ring mt-2 block w-full"
                        >
                            <option>TXT</option>
                            <option>A</option>
                            <option>AAAA</option>
                        </select>
                    </label>
                    <label className="field-label">
                        Frame bytes
                        <input
                            name="partSize"
                            type="number"
                            defaultValue={256}
                            min={128}
                            max={1024}
                            required
                            className="form-control focus-ring mt-2 block w-full"
                        />
                    </label>
                </div>
                <div className="flex flex-wrap items-center gap-4">
                    <button
                        type="submit"
                        disabled={!simulate}
                        className="focus-ring rounded bg-stone-900 px-4 py-2 text-white hover:bg-stone-700 disabled:opacity-40"
                    >
                        {simulate ? "Run simulation" : "Loading…"}
                    </button>
                    <p className="text-sm text-stone-500">
                        Runs in your browser. No DNS traffic is sent.
                    </p>
                </div>
            </form>
            {error && (
                <p role="alert" className="mt-6 text-sm text-red-700">
                    {error}
                </p>
            )}
            {result && (
                <section
                    aria-label="Simulation result"
                    className="mt-10 border-t border-stone-300 pt-6"
                >
                    <h2 className="font-medium">Result</h2>
                    <p role="status" className="mt-2 text-sm text-stone-600">
                        {result.inputBytes} input bytes ·{" "}
                        {result.packets.length} DNS responses ·{" "}
                        {result.wireBytes} DNS bytes
                    </p>
                    <div className="my-5 divide-y divide-stone-200 border-y border-stone-200">
                        {result.packets.map((packet, index) => (
                            <details key={packet.hex} className="py-3">
                                <summary className="focus-ring cursor-pointer text-sm">
                                    Packet {index + 1}
                                    <span className="ml-3 text-stone-500">
                                        {packet.bytes} bytes
                                    </span>
                                </summary>
                                <pre className="mt-3 max-h-64 overflow-auto whitespace-pre-wrap break-all text-xs leading-relaxed text-stone-600">
                                    {packet.hex}
                                </pre>
                            </details>
                        ))}
                    </div>
                    <h3 className="text-sm font-medium">Recovered message</h3>
                    <pre className="mt-2 whitespace-pre-wrap break-words font-mono text-sm">
                        {result.message || "(empty)"}
                    </pre>
                </section>
            )}
        </main>
    );
}

createRoot(document.getElementById("root")).render(<App />);
