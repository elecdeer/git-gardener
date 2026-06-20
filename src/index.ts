#!/usr/bin/env node
import { cli, define } from "gunshi";
import pkg from "../package.json" with { type: "json" };
import completion from "@gunshi/plugin-completion";

import { repoListCommand } from "./commands/repo/list.js";
import { repoRootCommand } from "./commands/repo/root.js";
import { repoCloneCommand } from "./commands/repo/clone.js";
import { repoMigrateCommand } from "./commands/repo/migrate.js";
import { wtListCommand } from "./commands/wt/list.js";
import { wtRootCommand } from "./commands/wt/root.js";
import { wtCreateCommand } from "./commands/wt/create.js";
import { wtSwitchCommand } from "./commands/wt/switch.js";
import { wtDeleteCommand } from "./commands/wt/delete.js";
import { wtPruneCommand } from "./commands/wt/prune.js";
import { wtMigrateCommand } from "./commands/wt/migrate.js";

/**
 * サブコマンドツリーの親コマンドが positional 付きで呼ばれた場合、
 * それは未知のサブコマンド名なのでエラー表示して終了する。
 * positional が無い場合は help 案内を出す。
 */
const handleParentRun = (parent: string, positionals: readonly string[]): void => {
  const unknown = positionals[1];
  if (unknown) {
    process.stderr.write(
      `gdn: unknown subcommand '${parent} ${unknown}'\nUse 'gdn ${parent} --help' to see available subcommands.\n`,
    );
    process.exit(1);
  }
  process.stdout.write(`Use 'gdn ${parent} --help' for usage information.\n`);
};

const repoCommand = define({
  name: "repo",
  description: "ghqライクなリポジトリのクローン・一覧管理",
  subCommands: {
    list: repoListCommand,
    root: repoRootCommand,
    clone: repoCloneCommand,
    migrate: repoMigrateCommand,
  },
  run: async (ctx) => {
    handleParentRun("repo", ctx.positionals);
  },
});

const wtCommand = define({
  name: "wt",
  description: "git worktreeの作成・切り替え・削除・整理",
  subCommands: {
    list: wtListCommand,
    root: wtRootCommand,
    create: wtCreateCommand,
    switch: wtSwitchCommand,
    delete: wtDeleteCommand,
    prune: wtPruneCommand,
    migrate: wtMigrateCommand,
  },
  run: async (ctx) => {
    handleParentRun("wt", ctx.positionals);
  },
});

const rootCommand = define({
  name: "git-gardener",
  description: "ghq と git worktree を統合したようなリポジトリ管理CLI",
  run: async () => {
    process.stdout.write("Use --help for usage information, or run a subcommand.\n");
  },
});

try {
  await cli(process.argv.slice(2), rootCommand, {
    name: "gdn",
    version: pkg.version,
    description: "ghq と git worktree を統合したようなリポジトリ管理CLI",
    renderHeader: null,
    subCommands: {
      repo: repoCommand,
      wt: wtCommand,
    },
    plugins: [completion()],
  });
} catch (err) {
  // gunshi はトップレベルの未知コマンドを `Command not found: <name>` で throw する。
  // Node のデフォルトスタックトレースを出さず、利用者向けメッセージに置き換える。
  if (err instanceof Error && err.message.startsWith("Command not found:")) {
    const name = err.message.slice("Command not found:".length).trim();
    process.stderr.write(
      `gdn: unknown command '${name}'\nUse 'gdn --help' to see available commands.\n`,
    );
    process.exit(1);
  }
  throw err;
}
