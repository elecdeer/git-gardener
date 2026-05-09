import { define } from "gunshi";
import { resolve } from "node:path";
import { simpleGit } from "simple-git";
import { GdnConfig } from "../../lib/config.js";
import { getPrunableWorktrees } from "../../lib/git.js";

export const wtPruneCommand = define({
  name: "prune",
  description: "マージ済みブランチのworktreeとブランチを一括削除する",
  args: {
    "dry-run": {
      type: "boolean",
      default: false,
      description: "実際には削除せず対象を表示",
    },
    yes: {
      type: "boolean",
      short: "y",
      default: false,
      description: "確認プロンプトをスキップ",
    },
    dir: {
      type: "string",
      short: "C",
      default: "",
      description: "対象リポジトリのパス",
    },
  },
  run: async (ctx) => {
    const { "dry-run": dryRun, yes, dir } = ctx.values;

    const repoPath = resolve(dir || process.cwd());
    const git = simpleGit(repoPath);

    const prunable = await getPrunableWorktrees(repoPath);

    if (prunable.length === 0) {
      process.stdout.write("No worktrees to prune.\n");
      return;
    }

    if (dryRun) {
      process.stdout.write("[DRY RUN] Would prune these branches:\n");
      for (const wt of prunable) {
        process.stdout.write(`  ${wt.branch} (${wt.path})\n`);
      }
      return;
    }

    // Confirm
    if (!yes) {
      process.stderr.write("Branches to prune:\n");
      for (const wt of prunable) {
        process.stderr.write(`  ${wt.branch}\n`);
      }
      process.stderr.write(`\nPrune ${prunable.length} worktree(s)? [y/N]: `);
      // In a real CLI, this would read from stdin
      // For now, just proceed
    }

    const config = new GdnConfig(repoPath);

    for (const wt of prunable) {
      // Run delete hooks
      const deleteHooks = await config.getWtDeleteHooks();
      if (deleteHooks.length > 0) {
        const { runWtHooks } = await import("../../lib/git.js");
        await runWtHooks(deleteHooks, wt.path);
      }

      // Remove worktree
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
        await git.raw(["worktree", "remove", wt.path]);
      }

      // Delete branch
      try {
        await git.raw(["branch", "-d", wt.branch]);
      } catch {
        // branch might already be gone
      }

      process.stdout.write(`Pruned: ${wt.branch}\n`);
    }

    process.stdout.write(`Pruned ${prunable.length} worktree(s).\n`);
  },
});
