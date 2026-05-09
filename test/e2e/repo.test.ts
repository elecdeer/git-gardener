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

/**
 * テスト実行ごとに変わる動的な値をプレースホルダーに置換する
 */
const normalize = (str: string): string =>
  str
    .replaceAll(testDir, "<testDir>")
    .replace(/\b[0-9a-f]{7}\b/g, "<hash>")
    .replace(/just now|\d+ (second|minute|hour|day)s? ago/g, "<time>")
    .replace(/\d{10,13}/g, "<timestamp>")
    .split("\n")
    .map((line) => line.trimEnd())
    .join("\n");

describe("gdn repo root", () => {
  it("should output the resolved gdn root path", async () => {
    const { stdout } = await gdn("repo root");
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"<testDir>/gdn-root"`);
  });
});

describe("gdn repo list", () => {
  it("should list repositories in tab-separated format", async () => {
    const { stdout } = await gdn("repo list --no-color --sort repo --reverse");
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "github.com/testuser/another-repo	<time>
      github.com/testuser/myrepo	<time>
      gitlab.com/otheruser/backend	<time>"
    `);
  });

  it("should filter by --limit", async () => {
    const { stdout } = await gdn("repo list --limit 1 --no-color --sort repo --reverse");
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"github.com/testuser/another-repo	<time>"`);
  });

  it("should output JSON with --json", async () => {
    const { stdout } = await gdn("repo list --json --sort repo --reverse");
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "[
        {
          "repo": "github.com/testuser/another-repo",
          "host": "github.com",
          "owner": "testuser",
          "name": "another-repo",
          "path": "<testDir>/gdn-root/github.com/testuser/another-repo",
          "branch": "main",
          "worktreeCount": 1,
          "commitMessage": "chore: setup",
          "updateAt": <timestamp>,
          "updateAtRelative": "<time>"
        },
        {
          "repo": "github.com/testuser/myrepo",
          "host": "github.com",
          "owner": "testuser",
          "name": "myrepo",
          "path": "<testDir>/gdn-root/github.com/testuser/myrepo",
          "branch": "main",
          "worktreeCount": 1,
          "commitMessage": "feat: initial commit",
          "updateAt": <timestamp>,
          "updateAtRelative": "<time>"
        },
        {
          "repo": "gitlab.com/otheruser/backend",
          "host": "gitlab.com",
          "owner": "otheruser",
          "name": "backend",
          "path": "<testDir>/gdn-root/gitlab.com/otheruser/backend",
          "branch": "main",
          "worktreeCount": 1,
          "commitMessage": "feat: backend init",
          "updateAt": <timestamp>,
          "updateAtRelative": "<time>"
        }
      ]"
    `);
  });

  it("should support custom columns with --column", async () => {
    const { stdout } = await gdn(
      "repo list --column host,owner,name --no-color --sort repo --reverse",
    );
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "github.com	testuser	another-repo
      github.com	testuser	myrepo
      gitlab.com	otheruser	backend"
    `);
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

    const git = simpleGit(clonedPath);
    const log = await git.log();
    expect(log.latest?.message).toContain("feat: hello");

    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"<testDir>/gdn-root/localhost/local/source-repo"`);
  });

  it("should error if destination already exists", async () => {
    const { stderr } = await gdn(`repo clone ${sourceRepo}`);
    expect(normalize(stderr.trim())).toMatchInlineSnapshot(`"Error: destination already exists: <testDir>/gdn-root/localhost/local/source-repo"`);
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
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "[DRY RUN] Would move:
        <testDir>/migrant-repo
        → <testDir>/gdn-root/github.com/testuser/migrant
      Config migrations:
        Skip ghq.root → gdn.root: gdn.root already set
        Skip wt.basedir → gdn.wtBasedir: gdn.wtBasedir already set
        Skip wt.hook → gdn.wtHook: gdn.wtHook already set
        Skip wt.deleteHook → gdn.wtDeleteHook: gdn.wtDeleteHook already set
        Skip wt.remover → gdn.wtRemover: gdn.wtRemover already set"
    `);
  });

  it("should migrate a repo to gdn structure", async () => {
    const { stdout } = await gdn(`repo migrate ${migrantRepo} --yes`);
    const newPath = stdout.trim();

    expect(existsSync(newPath)).toBe(true);
    expect(existsSync(migrantRepo)).toBe(false);

    const git = simpleGit(newPath);
    const log = await git.log();
    expect(log.latest?.message).toContain("feat: initial");

    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"<testDir>/gdn-root/github.com/testuser/migrant"`);
  });

  it("should report when repo is already in the correct location", async () => {
    // migrantRepo was moved to gdnRoot/.../migrant by the previous test
    const migratedPath = resolve(gdnRoot, "github.com", "testuser", "migrant");
    const { stderr } = await gdn(`repo migrate ${migratedPath} --yes`);
    expect(normalize(stderr.trim())).toMatchInlineSnapshot(`"Repository is already in the correct location."`);
  });

  it("should error when repo has no remote origin", async () => {
    const noRemoteDir = resolve(testDir, "no-remote-repo");
    await mkdir(noRemoteDir, { recursive: true });
    const git = simpleGit(noRemoteDir);
    await git.init();
    await writeFile(resolve(noRemoteDir, "file.txt"), "content");
    await git.add("file.txt");
    await git.raw("commit", "-m", "initial");
    await git.raw(["branch", "-m", "main"]);

    const { stderr } = await gdn(`repo migrate ${noRemoteDir} --yes`);
    expect(normalize(stderr.trim())).toMatchInlineSnapshot(`"Error: Cannot parse remote URL:"`);
  });

  it("should error when destination already exists", async () => {
    // Create a repo whose remote points to github.com/testuser/myrepo — which already exists in gdnRoot
    const conflictDir = resolve(testDir, "conflict-repo");
    await mkdir(conflictDir, { recursive: true });
    const git = simpleGit(conflictDir);
    await git.init();
    await git.raw(["remote", "add", "origin", "https://github.com/testuser/myrepo.git"]);
    await writeFile(resolve(conflictDir, "file.txt"), "content");
    await git.add("file.txt");
    await git.raw("commit", "-m", "initial");
    await git.raw(["branch", "-m", "main"]);

    const { stderr } = await gdn(`repo migrate ${conflictDir} --yes`);
    expect(normalize(stderr.trim())).toMatchInlineSnapshot(`"Error: destination already exists: <testDir>/gdn-root/github.com/testuser/myrepo"`);
  });
});
