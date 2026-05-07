import { describe, expect, it, vi } from "vitest";
import * as gitModule from "../src/git.js";

describe("git utilities", () => {
  describe("runGit", () => {
    it("should be a function", () => {
      expect(typeof gitModule.runGit).toBe("function");
    });
  });

  describe("getCurrentBranch", () => {
    it("should return the current branch name", async () => {
      const branch = await gitModule.getCurrentBranch();
      expect(typeof branch).toBe("string");
      expect(branch.length).toBeGreaterThan(0);
    });
  });

  describe("getMergedBranches", () => {
    it("should return an array of branch names", async () => {
      const branches = await gitModule.getMergedBranches();
      expect(Array.isArray(branches)).toBe(true);
      for (const b of branches) {
        expect(typeof b).toBe("string");
      }
    });
  });

  describe("getLocalBranches", () => {
    it("should return an array of BranchInfo objects", async () => {
      const branches = await gitModule.getLocalBranches();
      expect(Array.isArray(branches)).toBe(true);

      for (const branch of branches) {
        expect(typeof branch.name).toBe("string");
        expect(typeof branch.isRemote).toBe("boolean");
        expect(typeof branch.isMerged).toBe("boolean");
        expect(typeof branch.isCurrent).toBe("boolean");
        expect(typeof branch.lastCommitDate).toBe("string");
        expect(typeof branch.lastCommitMessage).toBe("string");
      }
    });

    it("should have exactly one current branch", async () => {
      const branches = await gitModule.getLocalBranches();
      const currentBranches = branches.filter((b) => b.isCurrent);
      expect(currentBranches.length).toBe(1);
    });
  });

  describe("getDefaultBranch", () => {
    it("should return a non-empty string", async () => {
      const branch = await gitModule.getDefaultBranch();
      expect(typeof branch).toBe("string");
      expect(branch.length).toBeGreaterThan(0);
    });
  });

  describe("deleteBranch", () => {
    it("should fail gracefully when branch does not exist", async () => {
      await expect(
        gitModule.deleteBranch("nonexistent-branch-that-does-not-exist"),
      ).rejects.toThrow();
    });
  });
});

describe("BranchInfo shape", () => {
  it("runGit output should be mocked for unit tests", async () => {
    const spy = vi.spyOn(gitModule, "runGit").mockResolvedValueOnce("mocked-branch");
    const result = await gitModule.runGit(["rev-parse", "--abbrev-ref", "HEAD"]);
    expect(result).toBe("mocked-branch");
    spy.mockRestore();
  });
});
