import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { goWasm } from "./web/vite-wasm";

export default defineConfig({
    root: "web",
    plugins: [goWasm(), react(), tailwindcss()],
    build: {
        outDir: "dist",
        emptyOutDir: true
    }
});
