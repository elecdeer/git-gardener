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
let mainRepoDir: string;
let remoteRepoDir: string;

beforeAll(async () => {
  testDir = await mkdtemp(join(tmpdir(), "gdn-e2e-wt-"));
  gdnRoot = resolve(testDir, "gdn-root");

  gitConfigGlobal = resolve(testDir, "gitconfig");
  await writeFile(gitConfigGlobal, `[gdn]\n\troot = ${gdnRoot}\n\tdefaultHost = github.com\n`);

  mainRepoDir = resolve(gdnRoot, "github.com", "testuser", "worktree-demo");
  await mkdir(mainRepoDir, { recursive: true });

  // prune がリモートの gone 判定を使うので、bare リポジトリを origin として用意する
  remoteRepoDir = resolve(testDir, "remote.git");
  await mkdir(remoteRepoDir, { recursive: true });
  await simpleGit(remoteRepoDir).raw(["init", "--bare", "--initial-branch=main"]);

  const git = simpleGit(mainRepoDir);
  await git.init();
  await writeFile(resolve(mainRepoDir, "README.md"), "# Worktree Demo");
  await git.add("README.md");
  await git.raw("commit", "-m", "feat: initial commit");
  await git.raw(["branch", "-m", "main"]);
  await git.raw(["remote", "add", "origin", remoteRepoDir]);
  await git.raw(["push", "-u", "origin", "main"]);

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
    .replace(/\d{10,13}/g, "<timestamp>")
    .split("\n")
    .map((line) => line.trimEnd())
    .join("\n");

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

  it("should support --column option to select specific fields", async () => {
    const { stdout } = await gdn(
      `wt list -C ${mainRepoDir} --no-color --sort branch --reverse --column branch,hash`,
    );
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "feature-a	<hash>
      main	<hash>"
    `);
  });

  it("should support --limit option", async () => {
    const { stdout } = await gdn(
      `wt list -C ${mainRepoDir} --no-color --sort branch --reverse --limit 1`,
    );
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"feature-a	/private<testDir>/feature-a	<hash>	<time>"`);
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
          "upstream": "origin/main",
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

  it("should create a worktree from a specific base branch", async () => {
    const { stdout, stderr } = await gdn(`wt create feature-based -C ${mainRepoDir} --base main`);
    expect(stderr).toBe("");

    const wtPath = stdout.trim();
    const git = simpleGit(wtPath);
    const branch = await git.raw(["rev-parse", "--abbrev-ref", "HEAD"]);
    expect(branch.trim()).toBe("feature-based");

    expect(normalize(wtPath)).toMatchInlineSnapshot(`"<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-based"`);

    // cleanup: delete the worktree so it doesn't affect subsequent prune tests
    await gdn(`wt delete feature-based -C ${mainRepoDir} --force`);
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

  it("should error when the specified branch worktree does not exist", async () => {
    const { stderr } = await gdn(`wt delete no-such-branch -C ${mainRepoDir}`);
    expect(normalize(stderr.trim())).toMatchInlineSnapshot(`"Error: worktree for branch 'no-such-branch' not found"`);
  });
});

describe("gdn wt prune", () => {
  beforeAll(async () => {
    // feature-a / feature-c を origin に push して upstream を張り、
    // リモートから ref を削除することで `[gone]` 状態を作る
    const git = simpleGit(mainRepoDir);
    await git.raw(["push", "-u", "origin", "feature-a"]);
    await git.raw(["push", "-u", "origin", "feature-c"]);

    const remoteGit = simpleGit(remoteRepoDir);
    await remoteGit.raw(["branch", "-D", "feature-a"]);
    await remoteGit.raw(["branch", "-D", "feature-c"]);
  });

  it("should show dry-run output", async () => {
    const { stdout } = await gdn(`wt prune -C ${mainRepoDir} --dry-run`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "[DRY RUN] Would prune these branches:
        feature-a (/private<testDir>/feature-a)
        feature-c (/private<testDir>/gdn-root/github.com/testuser/worktree-demo.wt/feature-c)"
    `);
  });

  it("should prune worktrees whose upstream branch was deleted on the remote", async () => {
    const { stdout } = await gdn(`wt prune -C ${mainRepoDir} --yes`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "Pruned: feature-a
      Pruned: feature-c
      Pruned 2 worktree(s)."
    `);
  });

  it("should output nothing-to-prune message when no gone-upstream worktrees remain", async () => {
    // All non-main worktrees have been pruned by the previous test
    const { stdout } = await gdn(`wt prune -C ${mainRepoDir} --dry-run`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"No worktrees to prune."`);
  });
});

describe("gdn wt migrate", () => {
  it("should show dry-run output", async () => {
    const { stdout } = await gdn(`wt migrate -C ${mainRepoDir} --dry-run`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`"All worktrees are already in the correct location."`);
  });
});

describe("gdn wt migrate (worktrees out of place)", () => {
  let migrateRepo: string;

  beforeAll(async () => {
    migrateRepo = resolve(gdnRoot, "github.com", "testuser", "migrate-wt-demo");
    await mkdir(migrateRepo, { recursive: true });

    const git = simpleGit(migrateRepo);
    await git.init();
    await writeFile(resolve(migrateRepo, "README.md"), "# Migrate WT Demo");
    await git.add("README.md");
    await git.raw("commit", "-m", "feat: initial");
    await git.raw(["branch", "-m", "main"]);

    // Create a worktree in a non-standard location (outside the expected basedir)
    await git.raw(["worktree", "add", "-b", "feature-x", resolve(testDir, "out-of-place-wt")]);
  });

  it("should show dry-run plan for misplaced worktrees", async () => {
    const { stdout } = await gdn(`wt migrate -C ${migrateRepo} --dry-run`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "[DRY RUN] Would move:
        /private<testDir>/out-of-place-wt
        → <testDir>/gdn-root/github.com/testuser/migrate-wt-demo.wt/feature-x"
    `);
  });

  it("should migrate worktrees to the correct basedir location", async () => {
    const { stdout } = await gdn(`wt migrate -C ${migrateRepo} --yes`);
    expect(normalize(stdout.trim())).toMatchInlineSnapshot(`
      "Moved: feature-x → <testDir>/gdn-root/github.com/testuser/migrate-wt-demo.wt/feature-x
      Migrated 1 worktree(s)."
    `);

    const expectedPath = resolve(migrateRepo, "..", "migrate-wt-demo.wt", "feature-x");
    expect(existsSync(expectedPath)).toBe(true);
    expect(existsSync(resolve(testDir, "out-of-place-wt"))).toBe(false);
  });
});
