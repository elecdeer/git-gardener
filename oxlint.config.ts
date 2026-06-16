import { defineConfig } from "oxlint";

export default defineConfig({
  options: {
    typeAware: true,
    typeCheck: true,
  },
  categories: {},
  rules: {
    "func-style": "error",
    "typescript/no-unnecessary-type-assertion": "error",
  },
});
