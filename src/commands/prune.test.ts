import { describe, expect, it, vi } from "vitest";

import * as gitModule from "../git.js";
import { pruneCommand } from "./prune.js";

describe("prune command", () => {
  it("should have a name of 'prune'", () => {
    expect(pruneCommand.name).toBe("prune");
  });

  it("should have a description", () => {
    expect(typeof pruneCommand.description).toBe("string");
    expect((pruneCommand.description ?? "").length).toBeGreaterThan(0);
  });

  it("should have dryRun, force, and keep args", () => {
    expect(pruneCommand.args).toBeDefined();
    expect(pruneCommand.args?.dryRun).toBeDefined();
    expect(pruneCommand.args?.force).toBeDefined();
    expect(pruneCommand.args?.keep).toBeDefined();
  });

  it("dryRun arg should default to false", () => {
    expect(pruneCommand.args?.dryRun?.default).toBe(false);
  });

  it("force arg should default to false", () => {
    expect(pruneCommand.args?.force?.default).toBe(false);
  });

  it("should output 'No branches to prune.' when nothing to delete", async () => {
    const logs: string[] = [];
    const consoleSpy = vi.spyOn(console, "log").mockImplementation((msg) => {
      logs.push(msg as string);
    });

    vi.spyOn(gitModule, "getDefaultBranch").mockResolvedValueOnce("main");
    vi.spyOn(gitModule, "getLocalBranches").mockResolvedValueOnce([
      {
        name: "main",
        isRemote: false,
        isMerged: true,
        isCurrent: true,
        lastCommitDate: "2024-01-01",
        lastCommitMessage: "Initial commit",
      },
    ]);

    if (pruneCommand.run) {
      const mockCtx = {
        values: { dryRun: false, force: false, keep: "" },
        positionals: [],
        rest: [],
        omitted: new Set<string>(),
        env: {
          name: "prune",
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
      await pruneCommand.run(mockCtx as any);
    }

    expect(logs).toContain("No branches to prune.");

    consoleSpy.mockRestore();
    vi.restoreAllMocks();
  });
});
