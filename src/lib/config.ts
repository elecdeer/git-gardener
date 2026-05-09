import { type SimpleGit, simpleGit } from "simple-git";
import { homedir } from "node:os";
import { resolve } from "node:path";

const expandHome = (path: string): string => {
  if (path.startsWith("~")) {
    return resolve(homedir(), path.slice(2));
  }
  return resolve(path);
};

const getRawConfig = async (
  git: SimpleGit,
  key: string,
  global?: boolean,
): Promise<string | undefined> => {
  const args = global ? ["config", "--get", "--global", key] : ["config", "--get", key];
  try {
    const result = await git.raw(args);
    return result.trim() || undefined;
  } catch {
    return undefined;
  }
};

const getAllRawConfig = async (
  git: SimpleGit,
  key: string,
  global?: boolean,
): Promise<string[]> => {
  const args = global ? ["config", "--get-all", "--global", key] : ["config", "--get-all", key];
  try {
    const result = await git.raw(args);
    return result
      .trim()
      .split("\n")
      .filter((v) => v.length > 0);
  } catch {
    return [];
  }
};

export class GdnConfig {
  private git: SimpleGit;

  constructor(cwd?: string) {
    this.git = cwd ? simpleGit(cwd) : simpleGit();
  }

  async getRoot(): Promise<string> {
    const roots = await getAllRawConfig(this.git, "gdn.root", true);
    if (roots.length > 1) {
      throw new Error("Multiple gdn.root config values are not allowed");
    }
    const root = roots[0] ?? "~/gdn";
    return expandHome(root);
  }

  async getDefaultHost(): Promise<string> {
    return (await getRawConfig(this.git, "gdn.defaultHost", true)) ?? "github.com";
  }

  async getPostCloneHooks(): Promise<string[]> {
    return getAllRawConfig(this.git, "gdn.postCloneHook", true);
  }

  async getWtBasedir(gitDirName?: string): Promise<string> {
    const local = await getRawConfig(this.git, "gdn.wtBasedir");
    if (local) {
      return expandWtBasedir(local, gitDirName);
    }
    const global = await getRawConfig(this.git, "gdn.wtBasedir", true);
    if (global) {
      return expandWtBasedir(global, gitDirName);
    }
    return `../${gitDirName ?? ""}.wt`;
  }

  async getWtHooks(): Promise<string[]> {
    const local = await getAllRawConfig(this.git, "gdn.wtHook");
    if (local.length > 0) return local;
    return getAllRawConfig(this.git, "gdn.wtHook", true);
  }

  async getWtDeleteHooks(): Promise<string[]> {
    const local = await getAllRawConfig(this.git, "gdn.wtDeleteHook");
    if (local.length > 0) return local;
    return getAllRawConfig(this.git, "gdn.wtDeleteHook", true);
  }

  async getWtRemover(): Promise<string> {
    return (
      (await getRawConfig(this.git, "gdn.wtRemover")) ??
      (await getRawConfig(this.git, "gdn.wtRemover", true)) ??
      "git worktree remove"
    );
  }
}

const expandWtBasedir = (basedir: string, gitDirName?: string): string => {
  let expanded = basedir;
  if (gitDirName !== undefined) {
    expanded = expanded.replace(/\{gitroot\}/g, gitDirName);
  }
  if (expanded.startsWith("~")) {
    return resolve(homedir(), expanded.slice(2));
  }
  return expanded;
};
