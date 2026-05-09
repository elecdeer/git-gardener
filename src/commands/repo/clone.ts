import { define } from "gunshi";
import { resolve } from "node:path";
import { existsSync } from "node:fs";
import { GdnConfig } from "../../lib/config.js";
import { cloneRepo } from "../../lib/git.js";

interface ParsedUrl {
  host: string;
  owner: string;
  name: string;
  url: string;
}

const parseUrl = (input: string, defaultHost: string): ParsedUrl => {
  // Handle https:// or git@ URLs
  if (input.startsWith("https://") || input.startsWith("http://")) {
    const parsed = new URL(input);
    const host = parsed.hostname;
    const pathParts = parsed.pathname.replace(/\.git$/, "").replace(/^\//, "").split("/");
    const owner = pathParts[0] ?? "";
    const name = pathParts[1] ?? "";
    return { host, owner, name, url: input };
  }

  if (input.startsWith("git@")) {
    const [hostPart, pathPart] = input.split(":");
    const host = hostPart?.replace("git@", "") ?? defaultHost;
    const pathClean = pathPart?.replace(/\.git$/, "") ?? "";
    const pathParts = pathClean.split("/");
    const owner = pathParts[0] ?? "";
    const name = pathParts[1] ?? "";
    return { host, owner, name, url: input };
  }

  // Handle host/user/project or user/project
  const parts = input.replace(/\.git$/, "").split("/");
  if (parts.length === 3) {
    return { host: parts[0]!, owner: parts[1]!, name: parts[2]!, url: `https://${input}` };
  }
  if (parts.length === 2) {
    return {
      host: defaultHost,
      owner: parts[0]!,
      name: parts[1]!,
      url: `https://${defaultHost}/${input}`,
    };
  }

  // Handle absolute or relative local paths
  if (input.startsWith("/") || input.startsWith(".")) {
    const name = input.replace(/\/$/, "").split("/").pop() ?? "repo";
    return { host: "localhost", owner: "local", name, url: input };
  }

  throw new Error(`Invalid repository URL format: ${input}`);
};

export const repoCloneCommand = define({
  name: "clone",
  description: "リポジトリを管理構造でクローンする",
  args: {
    url: {
      type: "positional",
      required: true,
      description: "クローンするリポジトリURL",
    },
    shallow: {
      type: "boolean",
      default: false,
      description: "shallow clone (--depth 1)",
    },
    branch: {
      type: "string",
      default: "",
      description: "特定ブランチをクローン",
    },
    submodules: {
      type: "boolean",
      default: false,
      description: "submoduleを再帰的にクローン",
    },
  },
  run: async (ctx) => {
    const { url: rawUrl, shallow, branch, submodules } = ctx.values;

    const config = new GdnConfig();
    const [root, defaultHost] = await Promise.all([config.getRoot(), config.getDefaultHost()]);

    const parsed = parseUrl(rawUrl, defaultHost);
    const destPath = resolve(root, parsed.host, parsed.owner, parsed.name);

    if (existsSync(destPath)) {
      process.stderr.write(`Error: destination already exists: ${destPath}\n`);
      process.exit(1);
    }

    process.stderr.write(`Cloning ${parsed.url} into ${destPath}...\n`);
    await cloneRepo(parsed.url, destPath, {
      shallow,
      ...(branch ? { branch } : {}),
      submodules,
    });

    // Run post-clone hooks
    const hooks = await config.getPostCloneHooks();
    if (hooks.length > 0) {
      const { runPostCloneHooks } = await import("../../lib/git.js");
      await runPostCloneHooks(hooks, destPath);
    }

    process.stdout.write(`${destPath}\n`);
  },
});
