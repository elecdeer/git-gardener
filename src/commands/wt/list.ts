import { define } from "gunshi";
import { resolve } from "node:path";
import { getWtInfos, type WtInfo } from "../../lib/git.js";
import { formatRelativeTime, formatTabular, formatJson } from "../../lib/format.js";
import type { WtColumnName } from "../../lib/field-defs.js";
import { wtListFields } from "../../lib/field-defs.js";

const VALID_COLUMNS = new Set<string>(wtListFields);

const parseColumns = (raw: string): WtColumnName[] => {
  return raw
    .split(",")
    .map((c) => c.trim())
    .filter((c): c is WtColumnName => VALID_COLUMNS.has(c));
};

const sortWorktrees = (wts: WtInfo[], field: string, reverse: boolean): WtInfo[] => {
  return [...wts].sort((a, b) => {
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

export const wtListCommand = define({
  name: "list",
  description: "対象リポジトリのworktree一覧をstdoutに出力する",
  args: {
    column: {
      type: "string",
      default: "branch,path,hash,updateAtRelative,tracking",
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
    dir: {
      type: "string",
      short: "C",
      default: "",
      description: "対象リポジトリのパス",
    },
    "include-branches": {
      type: "boolean",
      default: false,
      description: "worktreeに紐づいていないローカルブランチも含める",
    },
  },
  run: async (ctx) => {
    const {
      column,
      sort,
      reverse,
      limit,
      "no-color": noColor,
      json,
      dir,
    } = ctx.values as {
      column: string;
      sort: string;
      reverse: boolean;
      limit: string;
      "no-color": boolean;
      json: boolean;
      dir: string;
    };

    const repoPath = resolve(dir || process.cwd());
    const worktrees = await getWtInfos(repoPath);

    const limitNum = limit ? parseInt(limit, 10) : undefined;
    const sorted = sortWorktrees(worktrees, sort, reverse);
    const limited = limitNum ? sorted.slice(0, limitNum) : sorted;

    const rows = limited.map((w) => ({
      branch: w.branch,
      path: w.path,
      hash: w.hash,
      upstream: w.upstream,
      ahead: w.ahead,
      behind: w.behind,
      tracking: w.tracking,
      commitMessage: w.commitMessage,
      status: w.status,
      isMain: w.isMain,
      isCurrent: w.isCurrent,
      updateAt: w.updateAt,
      updateAtRelative: formatRelativeTime(w.updateAt),
    }));

    const cols = parseColumns(column);

    if (json) {
      process.stdout.write(formatJson(rows));
      process.stdout.write("\n");
    } else {
      process.stdout.write(formatTabular(rows, cols, { noColor }));
      process.stdout.write("\n");
    }
  },
});
