import { define } from "gunshi";
import { resolve, basename } from "node:path";
import { GdnConfig } from "../../lib/config.js";
import { getMainWorktreePath } from "../../lib/git.js";

export const wtRootCommand = define({
  name: "root",
  description: "gdn.wtBasedir を解決した絶対パスを出力する",
  args: {
    dir: {
      type: "string",
      short: "C",
      default: "",
      description: "対象リポジトリのパス（省略時はカレントディレクトリ）",
    },
  },
  run: async (ctx) => {
    const { dir } = ctx.values;
    const repoPath = await getMainWorktreePath(resolve(dir || process.cwd()));
    const repoName = basename(repoPath);

    const config = new GdnConfig(repoPath);
    const wtBasedir = await config.getWtBasedir(repoName);

    // wtBasedir can be relative (e.g. "../{gitroot}.wt")
    // Resolve relative to the repo directory
    const resolved = resolve(repoPath, wtBasedir);
    process.stdout.write(`${resolved}\n`);
  },
});
