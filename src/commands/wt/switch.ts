import { define } from "gunshi";
import { resolve, basename } from "node:path";
import { GdnConfig } from "../../lib/config.js";
import { switchWorktree } from "../../lib/git.js";

export const wtSwitchCommand = define({
  name: "switch",
  description: "指定ブランチのworktreeへ切り替える（存在しない場合は作成する）",
  args: {
    branch: {
      type: "positional",
      required: true,
      description: "切り替えるブランチ名",
    },
    base: {
      type: "string",
      default: "",
      description: "新規作成時の分岐元ブランチ",
    },
    dir: {
      type: "string",
      short: "C",
      default: "",
      description: "対象リポジトリのパス",
    },
  },
  run: async (ctx) => {
    const { branch, base, dir } = ctx.values;

    const repoPath = resolve(dir || process.cwd());
    const repoName = basename(repoPath);

    const config = new GdnConfig(repoPath);
    const wtBasedir = await config.getWtBasedir(repoName);
    const resolvedWtBasedir = resolve(repoPath, wtBasedir);

    const wtPath = await switchWorktree(repoPath, branch, resolvedWtBasedir, base || undefined);

    const hooks = await config.getWtHooks();
    if (hooks.length > 0) {
      const { runWtHooks } = await import("../../lib/git.js");
      await runWtHooks(hooks, wtPath);
    }

    process.stdout.write(`${wtPath}\n`);
  },
});
