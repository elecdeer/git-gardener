import { define } from "gunshi";
import { getCurrentBranch, getLocalBranches } from "../git.js";

export const listCommand = define({
  name: "list",
  description: "List local branches with status information",
  args: {
    merged: {
      type: "boolean",
      short: "m",
      default: false,
      description: "Show only merged branches",
    },
    all: {
      type: "boolean",
      short: "a",
      default: false,
      description: "Show all branches including protected ones",
    },
  },
  run: async (ctx) => {
    const { merged } = ctx.values;

    const branches = await getLocalBranches();
    const currentBranch = await getCurrentBranch();

    const filtered = merged === true ? branches.filter((b) => b.isMerged) : branches;

    if (filtered.length === 0) {
      console.log(merged === true ? "No merged branches found." : "No branches found.");
      return;
    }

    const maxNameLen = Math.max(...filtered.map((b) => b.name.length));

    for (const branch of filtered) {
      const prefix = branch.name === currentBranch ? "* " : "  ";
      const name = branch.name.padEnd(maxNameLen);
      const mergedTag = branch.isMerged ? "[merged] " : "         ";
      const date = branch.lastCommitDate.padEnd(10);
      console.log(`${prefix}${name}  ${mergedTag} ${date}  ${branch.lastCommitMessage}`);
    }
  },
});
