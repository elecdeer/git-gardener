#!/usr/bin/env node
import { cli, define } from "gunshi";

import { repoListCommand } from "./commands/repo/list.js";
import { repoRootCommand } from "./commands/repo/root.js";

const repoCommand = define({
  name: "repo",
  description: "ghqライクなリポジトリのクローン・一覧管理",
  subCommands: {
    list: repoListCommand,
    root: repoRootCommand,
  },
  run: async () => {
    console.log("Use gdn repo --help for usage information.");
  },
});

const wtCommand = define({
  name: "wt",
  description: "git worktreeの作成・切り替え・削除・整理",
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
  },
});
