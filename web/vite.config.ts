import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";
import path from "node:path";

export default defineConfig({
  plugins: [
    // Must run before @vitejs/plugin-react so the generated routeTree is in
    // place by the time React compiles.
    TanStackRouterVite({ target: "react", autoCodeSplitting: true }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      // Proxy /api/* to the Go scheduler running on the compose stack.
      //
      // changeOrigin MUST stay false. The browser treats
      // http://localhost:5173 as the single origin for everything (including
      // /api/* calls), so our SameSite=Strict session cookie is scoped to
      // localhost:5173. If we flip changeOrigin=true, Vite would rewrite
      // the Host header on the upstream request to localhost:8080 — any
      // server-side logic that reads Host (to set cookie Domain, to log,
      // etc.) would see a different origin than the browser's. Keeping
      // changeOrigin=false is the load-bearing detail for auth to work in
      // dev.
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: false,
      },
    },
  },
});
