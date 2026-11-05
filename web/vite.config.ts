import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// In development the Go server runs on :8095 and Vite proxies API and auth calls to it.
const backend = process.env.WHEREHOUSE_BACKEND ?? "http://localhost:8095";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": new URL("./src", import.meta.url).pathname } },
  server: {
    port: 5175,
    proxy: {
      "/api": backend,
      "/auth": backend,
      "/healthz": backend,
    },
  },
  build: { outDir: "dist", emptyOutDir: true, chunkSizeWarningLimit: 900 },
});
