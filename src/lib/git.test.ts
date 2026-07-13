import { describe, test, expect, vi, beforeEach } from "vitest";

const mockRaw = vi.fn();
const mockEnv = vi.fn();
const mockGit = {
  raw: mockRaw,
  env: mockEnv,
};
mockEnv.mockReturnValue(mockGit);

vi.mock("simple-git", () => ({
  simpleGit: vi.fn(() => mockGit),
}));

import { getMainWorktreePath, addWorktree, switchWorktree } from "./git.ts";

// git worktree list --porcelain の出力フォーマットを生成するヘルパー
const porcelain = (
  entries: Array<{ path: string; hash: string; branch: string }>,
): string =>
  entries
    .map(({ path, hash, branch }) =>
      `worktree ${path}\nHEAD ${hash}\nbranch refs/heads/${branch}\n`,
    )
    .join("\n");

describe("getMainWorktreePath", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("worktree 内から実行してもメインリポジトリのパスを返す", async () => {
    mockRaw.mockResolvedValue(
      porcelain([
        { path: "/project", hash: "abc1234", branch: "main" },
        { path: "/project.wt/feat/foo", hash: "def5678", branch: "feat/foo" },
      ]),
    );

    const result = await getMainWorktreePath("/project.wt/feat/foo");

    expect(result).toBe("/project");
  });

  test("メイン worktree から実行した場合はそのパスをそのまま返す", async () => {
    mockRaw.mockResolvedValue(
      porcelain([{ path: "/project", hash: "abc1234", branch: "main" }]),
    );

    const result = await getMainWorktreePath("/project");

    expect(result).toBe("/project");
  });

  test("worktree が複数あってもリストの先頭（メイン）を返す", async () => {
    mockRaw.mockResolvedValue(
      porcelain([
        { path: "/project", hash: "abc1234", branch: "main" },
        { path: "/project.wt/feat/a", hash: "aaa0001", branch: "feat/a" },
        { path: "/project.wt/feat/b", hash: "bbb0002", branch: "feat/b" },
      ]),
    );

    const result = await getMainWorktreePath("/project.wt/feat/b");

    expect(result).toBe("/project");
  });

  test("読み取り専用 Git コマンドでは optional lock を無効化する", async () => {
    mockRaw.mockResolvedValue(
      porcelain([{ path: "/project", hash: "abc1234", branch: "main" }]),
    );

    await getMainWorktreePath("/project");

    expect(mockEnv).toHaveBeenCalledWith(
      expect.objectContaining({ GIT_OPTIONAL_LOCKS: "0" }),
    );
  });
});

describe("switchWorktree", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("既存 worktree の検索では status を実行しない", async () => {
    mockRaw.mockResolvedValue(
      porcelain([
        { path: "/project", hash: "abc1234", branch: "main" },
        {
          path: "/project.wt/feat/existing",
          hash: "def5678",
          branch: "feat/existing",
        },
      ]),
    );

    const result = await switchWorktree(
      "/project",
      "feat/existing",
      "/project.wt",
    );

    expect({ result, calls: mockRaw.mock.calls }).toStrictEqual({
      result: "/project.wt/feat/existing",
      calls: [[["worktree", "list", "--porcelain"]]],
    });
  });
});

describe("addWorktree", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("ローカルにもリモートにも存在しない場合は optional lock を無効化せず -b で新規ブランチを作成する", async () => {
    mockRaw.mockImplementation(async (args: string[]) => {
      if (args[0] === "rev-parse") throw new Error("not a valid ref");
      if (args[0] === "branch") return "";
      return ""; // worktree add
    });

    await addWorktree("/project", "feat/new", "/project.wt/feat/new");

    expect(mockRaw).toHaveBeenCalledWith([
      "worktree",
      "add",
      "-b",
      "feat/new",
      "/project.wt/feat/new",
    ]);
    expect(mockEnv).not.toHaveBeenCalled();
  });

  test("ローカルにもリモートにも存在しない場合に baseBranch を分岐元として渡す", async () => {
    mockRaw.mockImplementation(async (args: string[]) => {
      if (args[0] === "rev-parse") throw new Error("not a valid ref");
      if (args[0] === "branch") return "";
      return "";
    });

    await addWorktree("/project", "feat/new", "/project.wt/feat/new", "main");

    expect(mockRaw).toHaveBeenCalledWith([
      "worktree",
      "add",
      "-b",
      "feat/new",
      "/project.wt/feat/new",
      "main",
    ]);
  });

  test("リモートにのみ存在する場合は -b なしで worktree を作成する", async () => {
    mockRaw.mockImplementation(async (args: string[]) => {
      if (args[0] === "rev-parse") throw new Error("not a valid ref");
      if (args[0] === "branch") return "  origin/feat/remote\n";
      return "";
    });

    await addWorktree("/project", "feat/remote", "/project.wt/feat/remote");

    expect(mockRaw).toHaveBeenCalledWith([
      "worktree",
      "add",
      "/project.wt/feat/remote",
      "feat/remote",
    ]);
    expect(mockRaw).not.toHaveBeenCalledWith(expect.arrayContaining(["-b"]));
  });

  test("リモートにのみ存在する場合は baseBranch を無視する", async () => {
    mockRaw.mockImplementation(async (args: string[]) => {
      if (args[0] === "rev-parse") throw new Error("not a valid ref");
      if (args[0] === "branch") return "  origin/feat/remote\n";
      return "";
    });

    await addWorktree(
      "/project",
      "feat/remote",
      "/project.wt/feat/remote",
      "main",
    );

    // リモートブランチが既に分岐元を持つため baseBranch は渡さない
    expect(mockRaw).toHaveBeenCalledWith([
      "worktree",
      "add",
      "/project.wt/feat/remote",
      "feat/remote",
    ]);
  });

  test("ローカルに存在する場合はリモート確認をスキップして -b で作成する", async () => {
    mockRaw.mockImplementation(async (args: string[]) => {
      if (args[0] === "rev-parse") return "abc1234\n";
      return "";
    });

    await addWorktree("/project", "feat/local", "/project.wt/feat/local");

    // branch -r は呼ばれない
    expect(mockRaw).not.toHaveBeenCalledWith(
      expect.arrayContaining(["branch"]),
    );
    expect(mockRaw).toHaveBeenCalledWith([
      "worktree",
      "add",
      "-b",
      "feat/local",
      "/project.wt/feat/local",
    ]);
  });
});
