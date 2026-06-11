import { defineConfig } from "tsup";

export default defineConfig({
  entry: ["index.ts"],
  format: "esm",
  target: "node22",
  outDir: "dist",
  clean: true,
  bundle: true,
  minify: true,
});
