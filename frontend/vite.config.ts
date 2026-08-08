import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

const API_TARGET = process.env.VITE_PROXY_TARGET ?? "http://localhost:8080";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: API_TARGET,
        changeOrigin: true,
      },
      // WebSocket events endpoint (plain ws upgrade, no token in URL — the
      // frontend authenticates via the subprotocol instead).
      "/ws": {
        target: API_TARGET,
        ws: true,
        changeOrigin: true,
      },
    },
  },
  build: {
    sourcemap: false,
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("node_modules")) {
            if (id.includes("framer-motion") || id.includes("motion")) return "motion";
            if (id.includes("@tanstack/react-query")) return "query";
            if (id.includes("@reduxjs/toolkit") || id.includes("react-redux")) return "state";
            if (id.includes("react-router") || id.includes("react-dom") || id.includes("react")) return "react";
          }
        },
      },
    },
  },
});
