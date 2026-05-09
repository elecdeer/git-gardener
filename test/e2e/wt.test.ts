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

  mainRepoDir = resolve(gdnRoot, "github.com", "testuser", "worktree-demo");
  await mkdir(mainRepoDir, { recursive: true });

  const git = simpleGit(mainRepoDir);
  await git.init();
  await writeFile(resolve(mainRepoDir, "README.md"), "# Worktree Demo");
  await git.add("README.md");
  await git.raw("commit", "-m", "feat: initial commit");
  await git.raw(["branch", "-m", "main"]);

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

/**
 * テスト実行ごとに変わる動的な値をプレースホルダーに置換する
 */
const normalize = (str: string): string =>
  str
    .replaceAll(testDir, "<testDir>")
    .replace(/\b[0-9a-f]{7}\b/g, "<hash>")
    .replace(/just now|\d+ (second|minute|hour|day)s? ago/g, "<time>")
    .replace(/\d{10,13}/g, "<timestamp>");

describe("gdn wt root", () => {
  it("should output the wt basedir", async () => {
    const { stdout } = await gdn(`wt root -C ${mainRepoDir}`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"<testDir>/gdn-root/github.com/testuser/worktree-demo.wt"`);
  });
});

describe("gdn wt list", () => {
  it("should list worktrees in tab-separated format", async () => {
    const { stdout } = await gdn(`wt list -C ${mainRepoDir} --no-color --sort branch --reverse`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "feature-a	/private<testDir>/feature-a	<hash>	<time>	
      main	/private<testDir>/gdn-root/github.com/testuser/worktree-demo	<hash>	<time>"
    `);
  });

  it("should support --json output", async () => {
    const { stdout } = await gdn(`wt list -C ${mainRepoDir} --json --sort branch --reverse`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "[
        {
          "branch": "feature-a",
          "path": "/private<testDir>/feature-a",
          "hash": "<hash>",
          "upstream": "",
          "ahead": 0,
          "behind": 0,
          "tracking": "",
          "commitMessage": "feat: initial commit",
          "status": "clean",
          "isMain": false,
          "isCurrent": false,
          "updateAt": <timestamp>,
          "updateAtRelative": "<time>"
        },
        {
          "branch": "main",
          "path": "/private<testDir>/gdn-root/github.com/testuser/worktree-demo",
          "hash": "<hash>",
          "upstream": "",
          "ahead": 0,
          "behind": 0,
          "tracking": "",
          "commitMessage": "feat: initial commit",
          "status": "clean",
          "isMain": true,
          "isCurrent": true,
          "updateAt": <timestamp>,
          "updateAtRelative": "<time>"
        }
      ]"
    `);
  });
});

describe("gdn wt create", () => {
  it("should create a new worktree", async () => {
    const { stdout } = await gdn(`wt create feature-b -C ${mainRepoDir}`);
    const wtPath = stdout.trim();

    const git = simpleGit(wtPath);
    const isRepo = await git.raw(["rev-parse", "--git-dir"]).catch(() => "");
    expect(isRepo).toBeTruthy();

    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-b"`);
  });
});

describe("gdn wt switch", () => {
  it("should switch to an existing worktree", async () => {
    const { stdout } = await gdn(`wt switch feature-a -C ${mainRepoDir}`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"/private<testDir>/feature-a"`);
  });

  it("should create and switch to a new worktree", async () => {
    const { stdout } = await gdn(`wt switch feature-c -C ${mainRepoDir}`);
    const wtPath = stdout.trim();

    const git = simpleGit(wtPath);
    const isRepo = await git.raw(["rev-parse", "--git-dir"]).catch(() => "");
    expect(isRepo).toBeTruthy();

    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-c"`);
  });
});

describe("gdn wt delete", () => {
  it("should delete a worktree and branch", async () => {
    const { stdout } = await gdn(`wt delete feature-b -C ${mainRepoDir} --force`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"Deleted worktree and branch: feature-b"`);

    const { stdout: listOut } = await gdn(`wt list -C ${mainRepoDir} --json`);
    const parsed = JSON.parse(listOut.trim());
    const found = parsed.find((w: { branch: string }) => w.branch === "feature-b");
    expect(found).toBeUndefined();
  });

  it("should refuse to delete default branch", async () => {
    const { stderr } = await gdn(`wt delete main -C ${mainRepoDir}`);
    expect(normalize(stderr.trim())).toMatchInlineSnapshot(`"Error: cannot delete default branch worktree: main"`);
  });
});

describe("gdn wt prune", () => {
  it("should show dry-run output", async () => {
    const { stdout } = await gdn(`wt prune -C ${mainRepoDir} --dry-run`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "[DRY RUN] Would prune these branches:
        feature-a (/private<testDir>/feature-a)
        feature-c (/private<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-c)"
    `);
  });

  it("should prune merged worktrees", async () => {
    const git = simpleGit(mainRepoDir);
    await git.raw(["merge", "feature-a"]);
    await git.raw(["checkout", "main"]);

    const { stdout } = await gdn(`wt prune -C ${mainRepoDir} --yes`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "Pruned: feature-a
      Pruned: feature-c
      Pruned 2 worktree(s)."
    `);
  });
});

describe("gdn wt migrate", () => {
  it("should show dry-run output", async () => {
    const { stdout } = await gdn(`wt migrate -C ${mainRepoDir} --dry-run`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"All worktrees are already in the correct location."`);
  });
});
