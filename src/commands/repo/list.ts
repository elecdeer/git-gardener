import { define } from "gunshi";
import { GdnConfig } from "../../lib/config.js";
import { getRepoInfos, type RepoInfo } from "../../lib/git.js";
import { formatRelativeTime, formatTabular, formatJson } from "../../lib/format.js";
import type { RepoColumnName } from "../../lib/field-defs.js";
import { repoListFields } from "../../lib/field-defs.js";

const VALID_COLUMNS = new Set<string>(repoListFields);

const parseColumns = (raw: string): RepoColumnName[] => {
  return raw
    .split(",")
    .map((c) => c.trim())
    .filter((c): c is RepoColumnName => VALID_COLUMNS.has(c));
};

const sortRepos = (repos: RepoInfo[], field: string, reverse: boolean): RepoInfo[] => {
  return [...repos].sort((a, b) => {
    const va = (a as unknown as Record<string, unknown>)[field];
    const vb = (b as unknown as Record<string, unknown>)[field];
    let cmp = 0;
    if (typeof va === "number" && typeof vb === "number") {
      cmp = vb - va;
    } else if (typeof va === "string" && typeof vb === "string") {
      cmp = vb.localeCompare(va);
    }
    return reverse ? -cmp : cmp;
  });
};

export const repoListCommand = define({
  name: "list",
  description: "管理下のリポジトリ一覧をstdoutに出力する",
  args: {
    column: {
      type: "string",
      default: "repo,updateAtRelative",
      description: "出力列をカンマ区切りで指定",
    },
    sort: {
      type: "string",
      default: "updateAt",
      description: "ソートフィールド",
    },
    reverse: {
      type: "boolean",
      default: false,
      description: "ソートを昇順にする",
    },
    limit: {
      type: "string",
      default: "",
      description: "上位n件のみ出力",
    },
    "no-color": {
      type: "boolean",
      default: false,
      description: "ANSIカラーなしで出力",
    },
    json: {
      type: "boolean",
      default: false,
      description: "JSON形式で出力",
    },
    "include-worktrees": {
      type: "boolean",
      default: false,
      description: "linked worktreeもリストに含める",
    },
  },
  run: async (ctx) => {
    const { column, sort, reverse, limit, "no-color": noColor, json } = ctx.values;

    const config = new GdnConfig();
    const root = await config.getRoot();

    const repos = await getRepoInfos(root);

    const limitNum = limit ? parseInt(limit, 10) : undefined;

    const sorted = sortRepos(repos, sort, reverse);
    const limited = limitNum ? sorted.slice(0, limitNum) : sorted;

    // Convert to Row format
    const rows = limited.map((r) => ({
      repo: r.repo,
      host: r.host,
      owner: r.owner,
      name: r.name,
      path: r.path,
      branch: r.branch,
      worktreeCount: r.worktreeCount,
      commitMessage: r.commitMessage,
      updateAt: r.updateAt,
      updateAtRelative: formatRelativeTime(r.updateAt),
    }));

    const cols = parseColumns(column);

    if (json) {
      process.stdout.write(formatJson(rows));
      process.stdout.write("\n");
    } else {
      process.stdout.write(
        formatTabular(rows, cols, { noColor }),
      );
      process.stdout.write("\n");
    }
  },
});
