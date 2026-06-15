import { describe, it, expect } from "vitest";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { resolve } from "node:path";

const execFileAsync = promisify(execFile);
const CLI = resolve(process.cwd(), "build/index");

interface RunResult {
  stdout: string;
  stderr: string;
  code: number;
}

const gdn = (args: string[]): Promise<RunResult> => {
  return execFileAsync(CLI, args, { encoding: "utf8" })
    .then((res) => ({ stdout: res.stdout, stderr: res.stderr, code: 0 }))
    .catch((err: NodeJS.ErrnoException & { stdout?: string; stderr?: string; code?: number }) => ({
      stdout: err.stdout ?? "",
      stderr: err.stderr ?? "",
      code: typeof err.code === "number" ? err.code : 1,
    }));
};

describe("unknown command handling", () => {
  it("should report a top-level unknown command without a stack trace", async () => {
    const { stdout, stderr, code } = await gdn(["foo"]);
    expect(stdout).toBe("");
    expect(stderr).toMatchInlineSnapshot(`
      "gdn: unknown command 'foo'
      Use 'gdn --help' to see available commands.
      "
    `);
    expect(code).toBe(1);
  });

  it("should report an unknown wt subcommand", async () => {
    const { stdout, stderr, code } = await gdn(["wt", "no-such"]);
    expect(stdout).toBe("");
    expect(stderr).toMatchInlineSnapshot(`
      "gdn: unknown subcommand 'wt no-such'
      Use 'gdn wt --help' to see available subcommands.
      "
    `);
    expect(code).toBe(1);
  });

  it("should report an unknown repo subcommand", async () => {
    const { stdout, stderr, code } = await gdn(["repo", "no-such"]);
    expect(stdout).toBe("");
    expect(stderr).toMatchInlineSnapshot(`
      "gdn: unknown subcommand 'repo no-such'
      Use 'gdn repo --help' to see available subcommands.
      "
    `);
    expect(code).toBe(1);
  });

  it("should still show help guidance when wt is called without a subcommand", async () => {
    const { stdout, code } = await gdn(["wt"]);
    expect(stdout).toMatchInlineSnapshot(`
      "Use 'gdn wt --help' for usage information.
      "
    `);
    expect(code).toBe(0);
  });
});
