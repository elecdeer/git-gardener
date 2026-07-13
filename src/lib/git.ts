import { type SimpleGit, simpleGit } from "simple-git";
import { resolve } from "node:path";
import { readdir, stat } from "node:fs/promises";

export interface RepoInfo {
  path: string;
  repo: string;
  host: string;
  owner: string;
  name: string;
  branch: string;
  worktreeCount: number;
  commitMessage: string;
  updateAt: number;
}

export interface WtInfo {
  branch: string;
  path: string;
  hash: string;
  upstream: string;
  ahead: number;
  behind: number;
  tracking: string;
  commitMessage: string;
  status: string;
  isMain: boolean;
  isCurrent: boolean;
  updateAt: number;
}

export interface WorktreeEntry {
  path: string;
  branch: string;
  hash: string;
}

const readOnlyGit = (dir: string): SimpleGit => {
  const git = simpleGit(dir);
  const env: Record<string, string> = { GIT_OPTIONAL_LOCKS: "0" };
  for (const [name, value] of Object.entries(process.env)) {
    if (name.startsWith("GIT_TRACE2") && value !== undefined) {
      env[name] = value;
    }
  }
  git.env(env);
  return git;
};

export const formatTracking = (ahead: number, behind: number): string => {
  const parts: string[] = [];
  if (ahead > 0) parts.push(`\u21e1${ahead}`);
  if (behind > 0) parts.push(`\u21e3${behind}`);
  return parts.join(" ");
};

export const getRepoInfos = async (root: string): Promise<RepoInfo[]> => {
  const repos: RepoInfo[] = [];

  try {
    const hosts = await readdir(root, { withFileTypes: true });
    for (const hostDir of hosts) {
      if (!hostDir.isDirectory() || hostDir.name.startsWith(".")) continue;
      const hostPath = resolve(root, hostDir.name);

      const owners = await readdir(hostPath, { withFileTypes: true });
      for (const ownerDir of owners) {
        if (!ownerDir.isDirectory() || ownerDir.name.startsWith(".")) continue;
        const ownerPath = resolve(hostPath, ownerDir.name);

        const projects = await readdir(ownerPath, { withFileTypes: true });
        for (const projDir of projects) {
          if (!projDir.isDirectory() || projDir.name.startsWith(".")) continue;
          const projPath = resolve(ownerPath, projDir.name);

          if (!(await isGitRepo(projPath))) continue;

          const repo = `${hostDir.name}/${ownerDir.name}/${projDir.name}`;
          const info = await getRepoInfo(projPath, repo);
          if (info) repos.push(info);
        }
      }
    }
  } catch {
    // root directory doesn't exist or is empty
  }

  return repos;
};

const isGitRepo = async (dir: string): Promise<boolean> => {
  try {
    const gitDir = resolve(dir, ".git");
    const s = await stat(gitDir);
    return s.isDirectory();
  } catch {
    return false;
  }
};

const getRepoInfo = async (dir: string, repoPath: string): Promise<RepoInfo | null> => {
  try {
    const git = readOnlyGit(dir);
    const [branchResult, logResult, worktreeResult] = await Promise.all([
      git.raw(["rev-parse", "--abbrev-ref", "HEAD"]),
      git.log({ maxCount: 1 }),
      git.raw(["worktree", "list", "--porcelain"]),
    ]);

    const branch = branchResult.trim() || "HEAD";
    const lastCommit = logResult.latest;
    const commitMessage = lastCommit?.message.split("\n")[0] ?? "";
    const updateAt = lastCommit ? new Date(lastCommit.date).getTime() / 1000 : 0;

    const worktreeCount = worktreeResult
      .trim()
      .split("\n")
      .filter((line) => line.startsWith("worktree ")).length;

    // Parse repo path: host/owner/name
    const parts = repoPath.split("/");
    const host = parts[0] ?? "";
    const owner = parts[1] ?? "";
    const name = parts[2] ?? "";

    return {
      path: dir,
      repo: repoPath,
      host,
      owner,
      name,
      branch,
      worktreeCount,
      commitMessage,
      updateAt,
    };
  } catch {
    return null;
  }
};

export const cloneRepo = async (
  url: string,
  destPath: string,
  opts: { shallow?: boolean; branch?: string; submodules?: boolean },
): Promise<void> => {
  const { shallow, branch, submodules } = opts;
  const cloneArgs: string[] = ["clone"];

  if (shallow) cloneArgs.push("--depth", "1");
  if (branch) cloneArgs.push("--branch", branch);
  if (submodules) cloneArgs.push("--recurse-submodules");

  cloneArgs.push(url, destPath);

  // Use bare simpleGit (no cwd) for clone
  await simpleGit().raw(cloneArgs);
};

export const getWtInfos = async (dir: string): Promise<WtInfo[]> => {
  const worktrees: WtInfo[] = [];

  try {
    const entries = await getWorktreeEntries(dir);

    const defaultBranch = await getDefaultBranch(dir);
    const currentWt = await getCurrentWorktree(dir);

    for (const entry of entries) {
      const info = await getWtInfo(entry.path, entry.branch, entry.hash, {
        defaultBranch,
        isCurrent: entry.path === currentWt,
      });
      if (info) worktrees.push(info);
    }
  } catch {
    return [];
  }

  return worktrees;
};

const parseWorktreePorcelain = (raw: string): WorktreeEntry[] => {
  const entries: WorktreeEntry[] = [];
  let current: Partial<WorktreeEntry> = {};

  for (const line of raw.split("\n")) {
    if (line.startsWith("worktree ")) {
      if (current.path) {
        entries.push({
          path: current.path,
          branch: current.branch ?? "HEAD",
          hash: current.hash ?? "",
        });
      }
      current = { path: line.slice("worktree ".length) };
    } else if (line.startsWith("branch ")) {
      current.branch = line.slice("branch ".length).replace("refs/heads/", "");
    } else if (line.startsWith("HEAD ")) {
      current.hash = line.slice("HEAD ".length).slice(0, 7);
    }
  }

  if (current.path) {
    entries.push({
      path: current.path,
      branch: current.branch ?? "HEAD",
      hash: current.hash ?? "",
    });
  }

  return entries;
};

export const getWorktreeEntries = async (dir: string): Promise<WorktreeEntry[]> => {
  try {
    const git = readOnlyGit(dir);
    const raw = await git.raw(["worktree", "list", "--porcelain"]);
    return parseWorktreePorcelain(raw);
  } catch {
    return [];
  }
};

const getDefaultBranch = async (dir: string): Promise<string> => {
  try {
    const git = readOnlyGit(dir);
    const result = await git.raw(["symbolic-ref", "--short", "refs/remotes/origin/HEAD"]);
    return result.trim().replace("origin/", "");
  } catch {
    try {
      // Fallback: check init.defaultBranch config
      const git = readOnlyGit(dir);
      const result = await git.raw(["config", "--get", "init.defaultBranch"]);
      const trimmed = result.trim();
      if (trimmed) return trimmed;
    } catch {
      // ignore
    }
    return "main";
  }
};

const getCurrentWorktree = async (dir: string): Promise<string> => {
  try {
    const git = readOnlyGit(dir);
    const result = await git.raw(["rev-parse", "--show-toplevel"]);
    return resolve(result.trim());
  } catch {
    return dir;
  }
};

const getWtInfo = async (
  wtPath: string,
  branch: string,
  hash: string,
  ctx: { defaultBranch: string; isCurrent: boolean },
): Promise<WtInfo | null> => {
  try {
    const wtGit = readOnlyGit(wtPath);
    const [logResult, statusResult, upstreamRaw] = await Promise.all([
      wtGit.log({ maxCount: 1 }),
      wtGit.status(),
      getUpstreamInfo(wtGit, branch),
    ]);

    const lastCommit = logResult.latest;
    const commitMessage = lastCommit?.message.split("\n")[0] ?? "";
    const updateAt = lastCommit ? new Date(lastCommit.date).getTime() / 1000 : 0;
    const isDirty = !statusResult.isClean();
    const status = isDirty ? "dirty" : "clean";

    return {
      branch,
      path: wtPath,
      hash,
      upstream: upstreamRaw.upstream,
      ahead: upstreamRaw.ahead,
      behind: upstreamRaw.behind,
      tracking: formatTracking(upstreamRaw.ahead, upstreamRaw.behind),
      commitMessage,
      status,
      isMain: branch === ctx.defaultBranch,
      isCurrent: ctx.isCurrent,
      updateAt,
    };
  } catch {
    return null;
  }
};

interface UpstreamInfo {
  upstream: string;
  ahead: number;
  behind: number;
}

const getUpstreamInfo = async (git: SimpleGit, branch: string): Promise<UpstreamInfo> => {
  try {
    const upstream = await git.raw(["rev-parse", "--abbrev-ref", `${branch}@{upstream}`]);
    const upstreamBranch = upstream.trim();

    const counts = await git.raw([
      "rev-list",
      "--left-right",
      "--count",
      `${branch}...${upstreamBranch}`,
    ]);

    const trimmed = counts.trim();
    const [aheadStr, behindStr] = trimmed.split("\t");
    return {
      upstream: upstreamBranch,
      ahead: (aheadStr ? parseInt(aheadStr, 10) : 0) || 0,
      behind: (behindStr ? parseInt(behindStr, 10) : 0) || 0,
    };
  } catch {
    return { upstream: "", ahead: 0, behind: 0 };
  }
};

/**
 * ブランチがローカルに存在するか確認する。
 */
const localBranchExists = async (git: SimpleGit, branch: string): Promise<boolean> => {
  try {
    await git.raw(["rev-parse", "--verify", `refs/heads/${branch}`]);
    return true;
  } catch {
    return false;
  }
};

/**
 * リモートトラッキングブランチが存在するか確認する。
 */
const remoteBranchExists = async (git: SimpleGit, branch: string): Promise<boolean> => {
  try {
    const result = await git.raw(["branch", "-r", "--list", `*/${branch}`]);
    return result.trim().length > 0;
  } catch {
    return false;
  }
};

export const addWorktree = async (
  dir: string,
  branch: string,
  wtDir: string,
  baseBranch?: string,
): Promise<string> => {
  const git = simpleGit(dir);

  const isLocal = await localBranchExists(git, branch);
  const isRemote = !isLocal && (await remoteBranchExists(git, branch));

  let args: string[];
  if (isRemote) {
    // リモートにのみ存在する場合は -b なしで実行。
    // git がリモートトラッキングブランチを元にローカルブランチを自動作成する。
    args = ["worktree", "add", wtDir, branch];
    await git.raw(args);
  } else {
    // ローカルに存在しない新規ブランチを作成する場合は -b を使う。
    args = ["worktree", "add", "-b", branch, wtDir];
    if (baseBranch) {
      args.push(baseBranch);
      // baseBranch 指定時は branch.autoSetupMerge=false を渡し、
      // 新ブランチが派生元の upstream（例: origin/main）を継承しないようにする。
      const gitNoTrack = simpleGit({ baseDir: dir, config: ["branch.autoSetupMerge=false"] });
      await gitNoTrack.raw(args);
    } else {
      await git.raw(args);
    }
  }

  return resolve(wtDir);
};

/**
 * 指定パスが属するリポジトリのメイン worktree パスを返す。
 * worktree 内から実行した場合でも正しいメインリポジトリのパスを返す。
 */
export const getMainWorktreePath = async (dir: string): Promise<string> => {
  const entries = await getWorktreeEntries(dir);
  return entries[0]?.path ?? dir;
};

export const switchWorktree = async (
  dir: string,
  branch: string,
  wtBasedir: string,
  baseBranch?: string,
): Promise<string> => {
  const existing = (await getWorktreeEntries(dir)).find((wt) => wt.branch === branch);

  if (existing) {
    return existing.path;
  }

  const wtDir = resolve(wtBasedir, branch);
  return addWorktree(dir, branch, wtDir, baseBranch);
};

/**
 * リモートで削除済み（upstream gone）のブランチを持つ worktree を返す。
 * `fetch` が true の場合は `git fetch --all --prune` を先に実行する。
 */
export const getPrunableWorktrees = async (
  dir: string,
  options: { fetch?: boolean } = {},
): Promise<WtInfo[]> => {
  const { fetch = true } = options;
  const git = simpleGit(dir);

  if (fetch) {
    try {
      await git.raw(["fetch", "--all", "--prune"]);
    } catch {
      // fetch の失敗（オフライン等）は致命的にしない。
      // 既存の追跡情報で判定を続行する。
    }
  }

  const refRaw = await git.raw([
    "for-each-ref",
    "--format=%(refname:short) %(upstream:track)",
    "refs/heads",
  ]);
  const goneBranches = new Set<string>();
  for (const line of refRaw.split("\n")) {
    if (!line.includes("[gone]")) continue;
    const branch = line.split(" ", 1)[0];
    if (branch) goneBranches.add(branch);
  }

  const worktrees = await getWtInfos(dir);
  return worktrees.filter((wt) => !wt.isMain && goneBranches.has(wt.branch));
};

export const isBareRepo = async (dir: string): Promise<boolean> => {
  try {
    const git = simpleGit(dir);
    const result = await git.raw(["rev-parse", "--is-bare-repository"]);
    return result.trim() === "true";
  } catch {
    return false;
  }
};

export const runPostCloneHooks = async (hooks: string[], cwd: string): Promise<void> => {
  for (const hook of hooks) {
    const { execFile } = await import("node:child_process");
    const { promisify } = await import("node:util");
    const execFileAsync = promisify(execFile);
    try {
      const [cmd, ...args] = hook.split(" ");
      if (cmd) {
        await execFileAsync(cmd, args, { cwd, encoding: "utf8" });
      }
    } catch {
      // Hook failures are non-fatal
    }
  }
};

export const runWtHooks = async (hooks: string[], cwd: string): Promise<void> => {
  for (const hook of hooks) {
    const { execFile } = await import("node:child_process");
    const { promisify } = await import("node:util");
    const execFileAsync = promisify(execFile);
    try {
      const [cmd, ...args] = hook.split(" ");
      if (cmd) {
        await execFileAsync(cmd, args, { cwd, encoding: "utf8" });
      }
    } catch {
      // Hook failures are non-fatal
    }
  }
};
