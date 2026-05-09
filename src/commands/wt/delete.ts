import { define } from "gunshi";
import { resolve } from "node:path";
import { simpleGit } from "simple-git";
import { GdnConfig } from "../../lib/config.js";
import { getWtInfos } from "../../lib/git.js";

export const wtDeleteCommand = define({
  name: "delete",
  description: "worktreeとブランチを削除する（デフォルトブランチは保護）",
  args: {
    branch: {
      type: "positional",
      required: true,
      description: "削除するブランチ名",
    },
    force: {
      type: "boolean",
      default: false,
      description: "未マージでも強制削除",
    },
    dir: {
      type: "string",
      short: "C",
      default: "",
      description: "対象リポジトリのパス",
    },
  },
  run: async (ctx) => {
    const { branch, force, dir } = ctx.values;

    const repoPath = resolve(dir || process.cwd());
    const git = simpleGit(repoPath);

    // Find the worktree
    const worktrees = await getWtInfos(repoPath);
    const wt = worktrees.find((w) => w.branch === branch);
    if (!wt) {
      process.stderr.write(`Error: worktree for branch '${branch}' not found\n`);
      process.exit(1);
    }

    // Protect default branch
    if (wt.isMain) {
      process.stderr.write(`Error: cannot delete default branch worktree: ${branch}\n`);
      process.exit(1);
    }

    // Run delete hooks
    const config = new GdnConfig(repoPath);
    const deleteHooks = await config.getWtDeleteHooks();
    if (deleteHooks.length > 0) {
      const { runWtHooks } = await import("../../lib/git.js");
      await runWtHooks(deleteHooks, wt.path);
    }

    // Use custom remover or default
    const remover = await config.getWtRemover();
    if (remover && remover !== "git worktree remove") {
      const { execFile } = await import("node:child_process");
      const { promisify } = await import("node:util");
      const execFileAsync = promisify(execFile);
      const [cmd, ...args] = remover.split(" ");
      if (cmd) {
        await execFileAsync(cmd, [...args, wt.path], { encoding: "utf8" });
      }
    } else {
      const removeArgs = ["worktree", "remove"];
      if (force) removeArgs.push("--force");
      removeArgs.push(wt.path);
      await git.raw(removeArgs);
    }

    // Delete the branch
    try {
      const branchFlag = force ? "-D" : "-d";
      await git.raw(["branch", branchFlag, branch]);
    } catch {
      // branch might already be gone
    }

    process.stdout.write(`Deleted worktree and branch: ${branch}\n`);
  },
});
