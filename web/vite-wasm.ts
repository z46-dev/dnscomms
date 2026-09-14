import { execFileSync } from "node:child_process";
import { copyFileSync, mkdirSync } from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { Plugin } from "vite";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const output = join(root, "web/public");

// Build with the runtime shipped by the same Go installation.
export function buildWasm() {
    mkdirSync(output, {
        recursive: true
    });

    execFileSync("go", ["build", "-o", join(output, "main.wasm"), "./web/src"], {
        cwd: root,
        env: {
            ...process.env,
            GOOS: "js",
            GOARCH: "wasm"
        },
        stdio: "pipe"
    });

    copyFileSync(join(execFileSync("go", ["env", "GOROOT"], {
        encoding: "utf8"
    }).trim(), "lib/wasm/wasm_exec.js"), join(output, "wasm_exec.js"));
}

export function goWasm(): Plugin {
    let timer: ReturnType<typeof setTimeout> | undefined;
    return {
        name: "go-wasm",
        buildStart() {
            buildWasm();
        },
        configureServer(server) {
            server.watcher.add(root);
            const onChange = (_event: string, file: string) => {
                if (!file.endsWith(".go") && !["go.mod", "go.sum"].includes(basename(file))) {
                    return;
                }

                clearTimeout(timer);
                timer = setTimeout(() => {
                    try {
                        buildWasm();
                        server.ws.send({
                            type: "full-reload"
                        });
                    } catch (error) {
                        const message = error instanceof Error ? "stderr" in error ? String(error.stderr || error.message) : error.message : String(error);
                        server.config.logger.error(message);
                        server.ws.send({
                            type: "error",
                            err: {
                                message: message,
                                stack: "",
                                plugin: "go-wasm"
                            }
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
