import { execFileSync } from "node:child_process";
import { copyFileSync, mkdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const output = join(root, "web/public");

// Build with the runtime shipped by the same Go installation.
export function buildWasm() {
    mkdirSync(output, { recursive: true });
    execFileSync(
        "go",
        ["build", "-o", join(output, "main.wasm"), "./web/src"],
        {
            cwd: root,
            env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
            stdio: "pipe"
        }
    );
    const goroot = execFileSync("go", ["env", "GOROOT"], {
        encoding: "utf8"
    }).trim();
    copyFileSync(
        join(goroot, "lib/wasm/wasm_exec.js"),
        join(output, "wasm_exec.js")
    );
}

export function goWasm() {
    let timer;
    return {
        name: "go-wasm",
        buildStart() {
            buildWasm();
        },
        configureServer(server) {
            server.watcher.add(root);
            const onChange = (_event, file) => {
                if (
                    !file.endsWith(".go") &&
                    !["go.mod", "go.sum"].includes(file.split("/").pop())
                )
                    return;
                clearTimeout(timer);
                timer = setTimeout(() => {
                    try {
                        buildWasm();
                        server.ws.send({ type: "full-reload" });
                    } catch (error) {
                        const message =
                            error.stderr?.toString() || error.message;
                        server.config.logger.error(message);
                        server.ws.send({
                            type: "error",
                            err: { message, stack: "", plugin: "go-wasm" }
                        });
                    }
                }, 150);
            };
            server.watcher.on("all", onChange);
            server.httpServer?.once("close", () => {
                clearTimeout(timer);
                server.watcher.off("all", onChange);
            });
        }
    };
}
