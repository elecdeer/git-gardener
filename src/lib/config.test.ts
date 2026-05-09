import { describe, it, expect, vi, beforeEach } from "vitest";

const mockRaw = vi.fn();

vi.mock("simple-git", () => ({
  simpleGit: vi.fn(() => ({
    raw: mockRaw,
  })),
}));

vi.mock("node:os", () => ({
  homedir: vi.fn(() => "/home/user"),
}));

import { GdnConfig } from "./config.ts";

describe("GdnConfig.getWtBasedir", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  /**
   * git config の値を返すモックを設定する
   */
  const setLocalConfig = (key: string, value: string) => {
    mockRaw.mockImplementation(async (args: string[]) => {
      if (args.includes("--global")) throw new Error("not found");
      if (args.includes(key)) return `${value}\n`;
      throw new Error("not found");
    });
  };

  it("チルダパスをホームディレクトリに展開する", async () => {
    setLocalConfig("gdn.wtBasedir", "~/worktrees");

    const config = new GdnConfig();
    const result = await config.getWtBasedir("myrepo");

    expect(result).toBe("/home/user/worktrees");
  });

  it("チルダのみのパスをホームディレクトリに展開する", async () => {
    setLocalConfig("gdn.wtBasedir", "~");

    const config = new GdnConfig();
    const result = await config.getWtBasedir("myrepo");

    expect(result).toBe("/home/user");
  });

  it("チルダパスで {gitroot} プレースホルダーを展開する", async () => {
    setLocalConfig("gdn.wtBasedir", "~/worktrees/{gitroot}.wt");

    const config = new GdnConfig();
    const result = await config.getWtBasedir("myrepo");

    expect(result).toBe("/home/user/worktrees/myrepo.wt");
  });

  it("設定がない場合はデフォルト値を返す", async () => {
    mockRaw.mockRejectedValue(new Error("not found"));

    const config = new GdnConfig();
    const result = await config.getWtBasedir("myrepo");

    expect(result).toBe("../myrepo.wt");
  });

  it("絶対パスはそのまま返す", async () => {
    setLocalConfig("gdn.wtBasedir", "/absolute/path");

    const config = new GdnConfig();
    const result = await config.getWtBasedir("myrepo");

    expect(result).toBe("/absolute/path");
  });
});
