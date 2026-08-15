import { defineConfig } from "vitest/config";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { svelteTesting } from "@testing-library/svelte/vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte(), svelteTesting()],
  server: {
    proxy: {
      "/api": "http://127.0.0.1:7777",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/vitest-setup.ts"],
    include: ["src/**/*.test.ts"],
  },
});
