import { defineConfig } from "tsdown";

export default defineConfig({
  entry: ["src/index.ts"],
  outDir: "dist",
  platform: "node",
  clean: true,
  exe: true,
  deps: {
    alwaysBundle: [/.*/],
  },
});
