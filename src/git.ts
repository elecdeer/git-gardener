import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

export interface BranchInfo {
  name: string;
  isRemote: boolean;
  isMerged: boolean;
  isCurrent: boolean;
  lastCommitDate: string;
  lastCommitMessage: string;
}

export const runGit = async (args: string[], cwd?: string): Promise<string> => {
  const { stdout } = await execFileAsync("git", args, {
    cwd: cwd ?? process.cwd(),
    encoding: "utf8",
  });
  return stdout.trim();
};

export const getCurrentBranch = async (cwd?: string): Promise<string> => {
  return runGit(["rev-parse", "--abbrev-ref", "HEAD"], cwd);
};

export const getMergedBranches = async (cwd?: string): Promise<string[]> => {
  const output = await runGit(["branch", "--merged", "HEAD"], cwd);
  return output
    .split("\n")
    .map((line) => line.trim().replace(/^\*\s*/, ""))
    .filter(Boolean);
};

export const getLocalBranches = async (cwd?: string): Promise<BranchInfo[]> => {
  const currentBranch = await getCurrentBranch(cwd);
  const mergedBranches = await getMergedBranches(cwd);

  const output = await runGit(
    ["for-each-ref", "--format=%(refname:short)|%(creatordate:short)|%(subject)", "refs/heads/"],
    cwd,
  );

  if (!output) return [];

  return output.split("\n").map((line) => {
    const parts = line.split("|");
    const name = parts[0] ?? "";
    const lastCommitDate = parts[1] ?? "";
    const lastCommitMessage = parts[2] ?? "";

    return {
      name,
      isRemote: false,
      isMerged: mergedBranches.includes(name),
      isCurrent: name === currentBranch,
      lastCommitDate,
      lastCommitMessage,
    };
  });
};

export const deleteBranch = async (branchName: string, force = false, cwd?: string): Promise<void> => {
  const flag = force ? "-D" : "-d";
  await runGit(["branch", flag, branchName], cwd);
};

export const getDefaultBranch = async (cwd?: string): Promise<string> => {
  try {
    const output = await runGit(["symbolic-ref", "--short", "refs/remotes/origin/HEAD"], cwd);
    return output.replace("origin/", "");
  } catch {
    return "main";
  }
};
