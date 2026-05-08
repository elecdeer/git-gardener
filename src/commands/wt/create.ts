import { define } from "gunshi";
import { resolve, basename } from "node:path";
import { GdnConfig } from "../../lib/config.js";
import { addWorktree } from "../../lib/git.js";

export const wtCreateCommand = define({
  name: "create",
  description: "worktreeを新規作成する",
  args: {
    branch: {
      type: "positional",
      required: true,
      description: "作成するブランチ名",
    },
    base: {
      type: "string",
      default: "",
      description: "分岐元ブランチ",
    },
    dir: {
      type: "string",
      short: "C",
      default: "",
      description: "対象リポジトリのパス",
    },
  },
  run: async (ctx) => {
    const { branch, base, dir } = ctx.values as {
      branch: string;
      base: string;
      dir: string;
    };

    const repoPath = resolve(dir || process.cwd());
    const repoName = basename(repoPath);

    const config = new GdnConfig(repoPath);
    const wtBasedir = await config.getWtBasedir(repoName);
    const resolvedWtBasedir = resolve(repoPath, wtBasedir);

    const wtPath = await addWorktree(repoPath, branch, resolve(resolvedWtBasedir, branch), base || undefined);

    const hooks = await config.getWtHooks();
    if (hooks.length > 0) {
      const { runWtHooks } = await import("../../lib/git.js");
      await runWtHooks(hooks, wtPath);
    }

    process.stdout.write(`${wtPath}\n`);
  },
});
