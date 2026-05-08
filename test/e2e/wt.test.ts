import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdtemp, writeFile, rm, mkdir } from "node:fs/promises";
import { resolve, join } from "node:path";
import { tmpdir } from "node:os";
import { simpleGit } from "simple-git";

const execFileAsync = promisify(execFile);
const CLI = resolve(process.cwd(), "build/index");

let testDir: string;
let gdnRoot: string;
let gitConfigGlobal: string;
let mainRepoDir: string;

beforeAll(async () => {
  testDir = await mkdtemp(join(tmpdir(), "gdn-e2e-wt-"));
  gdnRoot = resolve(testDir, "gdn-root");

  gitConfigGlobal = resolve(testDir, "gitconfig");
  await writeFile(gitConfigGlobal, `[gdn]\n\troot = ${gdnRoot}\n\tdefaultHost = github.com\n`);

  // Create a main repository under gdn root
  mainRepoDir = resolve(gdnRoot, "github.com", "testuser", "worktree-demo");
  await mkdir(mainRepoDir, { recursive: true });

  const git = simpleGit(mainRepoDir);
  await git.init();
  await writeFile(resolve(mainRepoDir, "README.md"), "# Worktree Demo");
  await git.add("README.md");
  await git.raw("commit", "-m", "feat: initial commit");
  await git.raw(["branch", "-m", "main"]);

  // Create a worktree
  await git.raw([
    "worktree",
    "add",
    "-b",
    "feature-a",
    resolve(testDir, "feature-a"),
  ]);
}, 60000);

afterAll(async () => {
  try {
    await rm(testDir, { recursive: true, force: true });
  } catch {
    // cleanup failure is ok
  }
});

const gdn = (args: string): Promise<{ stdout: string; stderr: string }> => {
  return execFileAsync(CLI, args.split(" "), {
    env: { ...process.env, GIT_CONFIG_GLOBAL: gitConfigGlobal },
    encoding: "utf8",
  }).catch((err: NodeJS.ErrnoException & { stdout?: string; stderr?: string }) => ({
    stdout: err.stdout ?? "",
    stderr: err.stderr ?? err.message ?? "",
  }));
};

describe("gdn wt root", () => {
  it("should output the wt basedir", async () => {
    const { stdout } = await gdn(`wt root -C ${mainRepoDir}`);
    const wtDir = stdout.trim();
    expect(wtDir).toBeTruthy();
    expect(wtDir.endsWith("worktree-demo.wt")).toBe(true);
  });
});

describe("gdn wt list", () => {
  it("should list worktrees in tab-separated format", async () => {
    const { stdout } = await gdn(`wt list -C ${mainRepoDir} --no-color`);
    const lines = stdout.trim().split("\n");
    expect(lines.length).toBeGreaterThanOrEqual(2); // main + feature-a

    for (const line of lines) {
      const parts = line.split("\t");
      expect(parts.length).toBeGreaterThanOrEqual(4);
      expect(parts[0]).toBeTruthy(); // branch name
    }
  });

  it("should support --json output", async () => {
    const { stdout } = await gdn(`wt list -C ${mainRepoDir} --json`);
    const parsed = JSON.parse(stdout.trim());
    expect(Array.isArray(parsed)).toBe(true);
    expect(parsed.length).toBeGreaterThanOrEqual(2);
    expect(parsed[0]).toHaveProperty("branch");
    expect(parsed[0]).toHaveProperty("path");
    expect(parsed[0]).toHaveProperty("hash");
  });
});

describe("gdn wt create", () => {
  it("should create a new worktree", async () => {
    const { stdout } = await gdn(`wt create feature-b -C ${mainRepoDir}`);
    const wtPath = stdout.trim();
    expect(wtPath).toBeTruthy();
    expect(wtPath).toContain("feature-b");

    // Verify the worktree exists
    const git = simpleGit(wtPath);
    const isRepo = await git.raw(["rev-parse", "--git-dir"]).catch(() => "");
    expect(isRepo).toBeTruthy();
  });
});

describe("gdn wt switch", () => {
  it("should switch to an existing worktree", async () => {
    // feature-a was already created in beforeAll
    const { stdout } = await gdn(`wt switch feature-a -C ${mainRepoDir}`);
    const wtPath = stdout.trim();
    expect(wtPath).toContain("feature-a");
  });

  it("should create and switch to a new worktree", async () => {
    const { stdout } = await gdn(`wt switch feature-c -C ${mainRepoDir}`);
    const wtPath = stdout.trim();
    expect(wtPath).toContain("feature-c");

    const git = simpleGit(wtPath);
    const isRepo = await git.raw(["rev-parse", "--git-dir"]).catch(() => "");
    expect(isRepo).toBeTruthy();
  });
});
