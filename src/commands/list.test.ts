import { describe, expect, it, vi } from "vitest";

import * as gitModule from "../git.js";
import { listCommand } from "./list.js";

describe("list command", () => {
  it("should have a name of 'list'", () => {
    expect(listCommand.name).toBe("list");
  });

  it("should have a description", () => {
    expect(typeof listCommand.description).toBe("string");
    expect((listCommand.description ?? "").length).toBeGreaterThan(0);
  });

  it("should have merged and all args", () => {
    expect(listCommand.args).toBeDefined();
    expect(listCommand.args?.merged).toBeDefined();
    expect(listCommand.args?.all).toBeDefined();
  });

  it("merged arg should default to false", () => {
    expect(listCommand.args?.merged?.default).toBe(false);
  });

  it("all arg should default to false", () => {
    expect(listCommand.args?.all?.default).toBe(false);
  });

  it("should list branches", async () => {
    const logs: string[] = [];
    const consoleSpy = vi.spyOn(console, "log").mockImplementation((msg) => {
      logs.push(msg as string);
    });

    vi.spyOn(gitModule, "getLocalBranches").mockResolvedValueOnce([
      {
        name: "main",
        isRemote: false,
        isMerged: true,
        isCurrent: true,
        lastCommitDate: "2024-01-01",
        lastCommitMessage: "Initial commit",
      },
      {
        name: "feature/my-branch",
        isRemote: false,
        isMerged: false,
        isCurrent: false,
        lastCommitDate: "2024-01-15",
        lastCommitMessage: "Add feature",
      },
    ]);
    vi.spyOn(gitModule, "getCurrentBranch").mockResolvedValueOnce("main");

    if (listCommand.run) {
      const mockCtx = {
        values: { merged: false, all: false },
        positionals: [],
        rest: [],
        omitted: new Set<string>(),
        env: {
          name: "list",
          version: undefined,
          description: undefined,
          locale: "en-US",
          leftMargin: 2,
          middleMargin: 2,
          usageOptionType: false,
          usageOptionValue: false,
        },
      };
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      await listCommand.run(mockCtx as any);
    }

    expect(logs.length).toBeGreaterThan(0);
    const mainLine = logs.find((l) => l.includes("main"));
    expect(mainLine).toBeDefined();
    expect(mainLine).toContain("*");

    consoleSpy.mockRestore();
    vi.restoreAllMocks();
  });

  it("should show 'No merged branches found.' when filtering merged on empty result", async () => {
    const logs: string[] = [];
    const consoleSpy = vi.spyOn(console, "log").mockImplementation((msg) => {
      logs.push(msg as string);
    });

    vi.spyOn(gitModule, "getLocalBranches").mockResolvedValueOnce([
      {
        name: "feature/wip",
        isRemote: false,
        isMerged: false,
        isCurrent: true,
        lastCommitDate: "2024-01-15",
        lastCommitMessage: "WIP",
      },
    ]);
    vi.spyOn(gitModule, "getCurrentBranch").mockResolvedValueOnce("feature/wip");

    if (listCommand.run) {
      const mockCtx = {
        values: { merged: true, all: false },
        positionals: [],
        rest: [],
        omitted: new Set<string>(),
        env: {
          name: "list",
          version: undefined,
          description: undefined,
          locale: "en-US",
          leftMargin: 2,
          middleMargin: 2,
          usageOptionType: false,
          usageOptionValue: false,
        },
      };
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      await listCommand.run(mockCtx as any);
    }

    expect(logs).toContain("No merged branches found.");

    consoleSpy.mockRestore();
    vi.restoreAllMocks();
  });
});
