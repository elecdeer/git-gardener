import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdtemp, writeFile, rm, mkdir } from "node:fs/promises";
import { existsSync } from "node:fs";
import { resolve, join } from "node:path";
import { tmpdir } from "node:os";
import { simpleGit } from "simple-git";

const execFileAsync = promisify(execFile);
const CLI = resolve(process.cwd(), "build/index");

let testDir: string;
let gdnRoot: string;
let gitConfigGlobal: string;

beforeAll(async () => {
  testDir = await mkdtemp(join(tmpdir(), "gdn-e2e-repo-"));
  gdnRoot = resolve(testDir, "gdn-root");

  gitConfigGlobal = resolve(testDir, "gitconfig");
  await writeFile(gitConfigGlobal, `[gdn]\n\troot = ${gdnRoot}\n\tdefaultHost = github.com\n`);

  const setupRepo = async (host: string, owner: string, name: string, msg: string) => {
    const dir = resolve(gdnRoot, host, owner, name);
    await mkdir(dir, { recursive: true });
    const g = simpleGit(dir);
    await g.init();
    await writeFile(resolve(dir, "README.md"), `# ${name}`);
    await g.add("README.md");
    await g.raw("commit", "-m", msg);
    await g.raw(["branch", "-m", "main"]);
  };

  await Promise.all([
    setupRepo("github.com", "testuser", "myrepo", "feat: initial commit"),
    setupRepo("github.com", "testuser", "another-repo", "chore: setup"),
    setupRepo("gitlab.com", "otheruser", "backend", "feat: backend init"),
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

    for (const line of lines) {
      const parts = line.split("\t");
      expect(parts.length).toBe(2);
      expect(parts[0]).toBeTruthy();
      expect(parts[1]).toBeTruthy();
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
    expect(parts[0]).toBeTruthy();
    expect(parts[1]).toBeTruthy();
    expect(parts[2]).toBeTruthy();
  });
});

describe("gdn repo clone", () => {
  let sourceRepo: string;

  beforeAll(async () => {
    sourceRepo = resolve(testDir, "source-repo");
    await mkdir(sourceRepo, { recursive: true });
    const git = simpleGit(sourceRepo);
    await git.init();
    await writeFile(resolve(sourceRepo, "hello.txt"), "Hello World");
    await git.add("hello.txt");
    await git.raw("commit", "-m", "feat: hello");
    await git.raw(["branch", "-m", "main"]);
  });

  it("should clone a local repository", async () => {
    const { stdout } = await gdn(`repo clone ${sourceRepo}`);
    const clonedPath = stdout.trim();
    expect(clonedPath).toContain(gdnRoot);

    const git = simpleGit(clonedPath);
    const log = await git.log();
    expect(log.latest?.message).toContain("feat: hello");
  });

  it("should error if destination already exists", async () => {
    const { stderr } = await gdn(`repo clone ${sourceRepo}`);
    expect(stderr).toContain("already exists");
  });
});

describe("gdn repo migrate", () => {
  let migrantRepo: string;

  beforeAll(async () => {
    migrantRepo = resolve(testDir, "migrant-repo");
    await mkdir(migrantRepo, { recursive: true });
    const git = simpleGit(migrantRepo);
    await git.init();
    await git.raw(["remote", "add", "origin", "https://github.com/testuser/migrant.git"]);
    await writeFile(resolve(migrantRepo, "README.md"), "# Migrant");
    await git.add("README.md");
    await git.raw("commit", "-m", "feat: initial");
    await git.raw(["branch", "-m", "main"]);
  });

  it("should show dry-run plan", async () => {
    const { stdout } = await gdn(`repo migrate ${migrantRepo} --dry-run --yes`);
    expect(stdout).toContain("DRY RUN");
    expect(stdout).toContain("migrant-repo");
    expect(stdout).toContain("github.com");
  });

  it("should migrate a repo to gdn structure", async () => {
    const { stdout } = await gdn(`repo migrate ${migrantRepo} --yes`);
    const newPath = stdout.trim();
    expect(newPath).toContain(gdnRoot);
    expect(newPath).toContain("github.com/testuser/migrant");

    expect(existsSync(newPath)).toBe(true);
    expect(existsSync(migrantRepo)).toBe(false);

    const git = simpleGit(newPath);
    const log = await git.log();
    expect(log.latest?.message).toContain("feat: initial");
  });
});
