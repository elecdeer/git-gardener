import { define } from "gunshi";

import { deleteBranch, getDefaultBranch, getLocalBranches } from "../git.js";

export const pruneCommand = define({
  name: "prune",
  description: "Remove merged branches from the local repository",
  args: {
    dryRun: {
      type: "boolean",
      short: "n",
      default: false,
      description: "Show what would be deleted without actually deleting",
    },
    force: {
      type: "boolean",
      short: "f",
      default: false,
      description: "Force delete branches even if not fully merged",
    },
    keep: {
      type: "string",
      short: "k",
      default: "",
      description: "Comma-separated list of branch names to keep (e.g. main,develop)",
    },
  },
  run: async (ctx) => {
    const { dryRun, force, keep } = ctx.values;
    const keepBranches =
      typeof keep === "string" && keep ? keep.split(",").map((b) => b.trim()) : [];

    const defaultBranch = await getDefaultBranch();
    const protectedBranches = new Set([
      "main",
      "master",
      "develop",
      defaultBranch,
      ...keepBranches,
    ]);

    const branches = await getLocalBranches();
    const toDelete = branches.filter(
      (b) => !b.isCurrent && (b.isMerged || force === true) && !protectedBranches.has(b.name),
    );

    if (toDelete.length === 0) {
      console.log("No branches to prune.");
      return;
    }

    if (dryRun === true) {
      console.log("Branches that would be pruned (dry run):");
    } else {
      console.log("Pruning branches:");
    }

    for (const branch of toDelete) {
      if (dryRun === true) {
        console.log(`  - ${branch.name} (${branch.lastCommitDate}: ${branch.lastCommitMessage})`);
      } else {
        await deleteBranch(branch.name, force === true);
        console.log(`  ✓ Deleted ${branch.name}`);
      }
    }

    if (dryRun !== true) {
      console.log(`\nPruned ${toDelete.length} branch${toDelete.length !== 1 ? "es" : ""}.`);
    }
  },
});
