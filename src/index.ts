#!/usr/bin/env node
import { cli, define } from "gunshi";

import { listCommand } from "./commands/list.js";
import { pruneCommand } from "./commands/prune.js";

const rootCommand = define({
  name: "git-gardener",
  description: "A CLI tool for managing and pruning git branches",
  run: async () => {
    console.log("Use --help for usage information, or run a subcommand.");
  },
});

await cli(process.argv.slice(2), rootCommand, {
  name: "git-gardener",
  version: "0.1.0",
  description: "A CLI tool for managing and pruning git branches",
  subCommands: {
    list: listCommand,
    prune: pruneCommand,
  },
});
