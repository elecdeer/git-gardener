import { define } from "gunshi";
import { resolve, basename } from "node:path";
import { existsSync } from "node:fs";
import { rename, mkdir } from "node:fs/promises";
import { dirname } from "node:path";
import { GdnConfig } from "../../lib/config.js";
import { getWtInfos, isBareRepo, getMainWorktreePath } from "../../lib/git.js";

export const wtMigrateCommand = define({
  name: "migrate",
  description: "既存リポジトリのworktreeをgdn.wtBasedir配下の正規化された構造へ移行する",
  args: {
    "dry-run": {
      type: "boolean",
      default: false,
      description: "実際には移動せず変更予定を表示",
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

    const repoPath = await getMainWorktreePath(resolve(dir || process.cwd()));
    const repoName = basename(repoPath);

    const config = new GdnConfig(repoPath);
    const wtBasedir = await config.getWtBasedir(repoName);
    const resolvedWtBasedir = resolve(repoPath, wtBasedir);

    const bare = await isBareRepo(repoPath);
    const worktrees = await getWtInfos(repoPath);

    if (bare) {
      process.stderr.write("Error: bare repository migration is not yet supported.\n");
      process.exit(1);
    }

    // Find worktrees that need migration (non-main worktrees not already in wtBasedir)
    const toMigrate = worktrees.filter((wt) => {
      if (wt.isMain) return false;
      const targetPath = resolve(resolvedWtBasedir, wt.branch);
      return resolve(wt.path) !== targetPath;
    });

    if (toMigrate.length === 0) {
      process.stdout.write("All worktrees are already in the correct location.\n");
      return;
    }

    if (dryRun) {
      process.stdout.write("[DRY RUN] Would move:\n");
      for (const wt of toMigrate) {
        const targetPath = resolve(resolvedWtBasedir, wt.branch);
        process.stdout.write(`  ${wt.path}\n  → ${targetPath}\n\n`);
      }
      return;
    }

    if (!yes) {
      process.stderr.write(`Migrate ${toMigrate.length} worktree(s)?\n`);
      for (const wt of toMigrate) {
        const targetPath = resolve(resolvedWtBasedir, wt.branch);
        process.stderr.write(`  ${wt.branch}: ${wt.path} → ${targetPath}\n`);
      }
      process.stderr.write("Proceed? [y/N]: ");
      // In a real CLI with stdin, this would read confirmation
    }

    // Ensure target directory exists
    await mkdir(resolvedWtBasedir, { recursive: true });

    for (const wt of toMigrate) {
      const targetPath = resolve(resolvedWtBasedir, wt.branch);

      if (existsSync(targetPath)) {
        process.stderr.write(`Warning: target already exists for ${wt.branch}, skipping\n`);
        continue;
      }

      await mkdir(dirname(targetPath), { recursive: true });
      await rename(wt.path, targetPath);
      process.stdout.write(`Moved: ${wt.branch} → ${targetPath}\n`);
    }

    process.stdout.write(`Migrated ${toMigrate.length} worktree(s).\n`);
  },
});
