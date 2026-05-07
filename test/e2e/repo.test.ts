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

beforeAll(async () => {
  // Create temp directories
  testDir = await mkdtemp(join(tmpdir(), "gdn-e2e-repo-"));
  gdnRoot = resolve(testDir, "gdn-root");

  // Create a temp git config file for global settings
  gitConfigGlobal = resolve(testDir, "gitconfig");
  await writeFile(gitConfigGlobal, `[gdn]\n\troot = ${gdnRoot}\n\tdefaultHost = github.com\n`);

  // Create sample repos under gdn root structure
  const repoDir = resolve(gdnRoot, "github.com", "testuser", "myrepo");
  await mkdir(repoDir, { recursive: true });

  const git = simpleGit(repoDir);
  await git.init();
  await writeFile(resolve(repoDir, "README.md"), "# My Repo");
  await git.add("README.md");
  await git.commit("feat: initial commit");
  await git.raw(["branch", "-m", "main"]);

  // Create another repo
  const repoDir2 = resolve(gdnRoot, "github.com", "testuser", "another-repo");
  await mkdir(repoDir2, { recursive: true });

  const git2 = simpleGit(repoDir2);
  await git2.init();
  await writeFile(resolve(repoDir2, "README.md"), "# Another");
  await git2.add("README.md");
  await git2.commit("chore: setup");

  // Create a third repo under a different host
  const repoDir3 = resolve(gdnRoot, "gitlab.com", "otheruser", "backend");
  await mkdir(repoDir3, { recursive: true });

  const git3 = simpleGit(repoDir3);
  await git3.init();
  await writeFile(resolve(repoDir3, "README.md"), "# Backend");
  await git3.add("README.md");
  await git3.commit("feat: backend init");
}, 30000);

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
  });
};

describe("gdn repo root", () => {
  it("should output the resolved gdn root path", async () => {
    const { stdout } = await gdn("repo root");
    expect(stdout.trim()).toBe(gdnRoot);
  });
});

describe("gdn repo list", () => {
  it("should list repositories in tab-separated format", async () => {
    const { stdout } = await gdn("repo list --no-color");
    const lines = stdout.trim().split("\n");
    expect(lines.length).toBe(3);

    // Each line should have tab-separated columns (repo, updateAtRelative)
    for (const line of lines) {
      const parts = line.split("\t");
      expect(parts.length).toBe(2);
      expect(parts[0]).toBeTruthy(); // repo path
      expect(parts[1]).toBeTruthy(); // relative time
    }
  });

  it("should filter by --limit", async () => {
    const { stdout } = await gdn("repo list --limit 1 --no-color");
    const lines = stdout.trim().split("\n");
    expect(lines.length).toBe(1);
  });

  it("should output JSON with --json", async () => {
    const { stdout } = await gdn("repo list --json");
    const parsed = JSON.parse(stdout.trim());
    expect(Array.isArray(parsed)).toBe(true);
    expect(parsed.length).toBeGreaterThanOrEqual(2);
    expect(parsed[0]).toHaveProperty("repo");
    expect(parsed[0]).toHaveProperty("host");
    expect(parsed[0]).toHaveProperty("owner");
  });

  it("should support custom columns with --column", async () => {
    const { stdout } = await gdn(
      "repo list --column host,owner,name --no-color --limit 1",
    );
    const lines = stdout.trim().split("\n");
    expect(lines.length).toBe(1);
    const parts = lines[0]!.split("\t");
    expect(parts.length).toBe(3);
    expect(parts[0]).toBeTruthy(); // host
    expect(parts[1]).toBeTruthy(); // owner
    expect(parts[2]).toBeTruthy(); // name
  });
});
