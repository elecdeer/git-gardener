import { define } from "gunshi";
import { GdnConfig } from "../../lib/config.js";

export const repoRootCommand = define({
  name: "root",
  description: "gdn.root を解決した絶対パスを出力する",
  run: async () => {
    const config = new GdnConfig();
    const root = await config.getRoot();
    process.stdout.write(`${root}\n`);
  },
});
