import { defineConfig } from "tsdown";

export default defineConfig({
  entry: ["src/index.ts"],
  outDir: "dist",
  platform: "node",
  clean: true,
  exe: {
    outDir: "build",
    targets: [
      { platform: "darwin", arch: "arm64", nodeVersion: "latest" },
      { platform: "darwin", arch: "x64", nodeVersion: "latest" },
    ],
  },
  deps: {
    alwaysBundle: [/.*/],
  },
});
