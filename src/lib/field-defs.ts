export type Row = Record<string, unknown>;

const fmt = (v: unknown, fallback = ""): string => {
  if (v == null) return fallback;
  switch (typeof v) {
    case "string":
      return v;
    case "number":
    case "bigint":
    case "boolean":
      return v.toString();
    case "symbol":
      return v.description ?? "Symbol()";
    default:
      return JSON.stringify(v);
  }
};

export const repoListFields = [
  "repo",
  "host",
  "owner",
  "name",
  "path",
  "branch",
  "worktreeCount",
  "commitMessage",
  "updateAt",
  "updateAtRelative",
] as const;

export const wtListFields = [
  "branch",
  "path",
  "hash",
  "upstream",
  "ahead",
  "behind",
  "tracking",
  "commitMessage",
  "status",
  "isMain",
  "isCurrent",
  "updateAt",
  "updateAtRelative",
] as const;

export type RepoColumnName = (typeof repoListFields)[number];
export type WtColumnName = (typeof wtListFields)[number];
export type ColumnName = RepoColumnName | WtColumnName;

export interface FieldDef {
  color: string | ((row: Row) => string) | null;
  format: (value: unknown, row: Row) => string;
}

export const fieldDefs: Record<string, FieldDef> = {
  repo: { color: null, format: (v) => fmt(v) },
  host: { color: null, format: (v) => fmt(v) },
  owner: { color: null, format: (v) => fmt(v) },
  name: { color: null, format: (v) => fmt(v) },
  path: { color: null, format: (v) => fmt(v) },
  branch: { color: "cyan", format: (v) => fmt(v) },
  worktreeCount: { color: null, format: (v) => fmt(v, "0") },
  commitMessage: { color: "yellow", format: (v) => fmt(v) },
  updateAt: { color: null, format: (v) => fmt(v) },
  updateAtRelative: { color: "green", format: (v) => fmt(v) },
  hash: { color: "yellow", format: (v) => fmt(v) },
  upstream: { color: "magenta", format: (v) => fmt(v) },
  ahead: { color: "green", format: (v) => fmt(v, "0") },
  behind: { color: "red", format: (v) => fmt(v, "0") },
  tracking: { color: null, format: (v) => fmt(v) },
  status: {
    color: (row: Row) => (row["status"] === "dirty" ? "red" : "green"),
    format: (v) => fmt(v, "clean"),
  },
  isMain: { color: null, format: (v) => (v ? "\u2713" : "") },
  isCurrent: { color: null, format: (v) => (v ? "\u2713" : "") },
};
