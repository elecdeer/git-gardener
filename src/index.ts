#!/usr/bin/env node
import { cli, define } from "gunshi";

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
import { completeCommand } from "./commands/complete.js";

const repoCommand = define({
  name: "repo",
  description: "ghqライクなリポジトリのクローン・一覧管理",
  subCommands: {
    list: repoListCommand,
    root: repoRootCommand,
    clone: repoCloneCommand,
    migrate: repoMigrateCommand,
  },
  run: async () => {
    console.log("Use gdn repo --help for usage information.");
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
  run: async () => {
    console.log("Use gdn wt --help for usage information.");
  },
});

const rootCommand = define({
  name: "git-gardener",
  description: "ghq と git worktree を統合したようなリポジトリ管理CLI",
  run: async () => {
    console.log("Use --help for usage information, or run a subcommand.");
  },
});

await cli(process.argv.slice(2), rootCommand, {
  name: "gdn",
  version: "0.1.0",
  description: "ghq と git worktree を統合したようなリポジトリ管理CLI",
  renderHeader: null,
  subCommands: {
    repo: repoCommand,
    wt: wtCommand,
    complete: completeCommand,
  },
});
