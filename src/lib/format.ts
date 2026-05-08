import { type ColumnName, type Row, fieldDefs } from "./field-defs.js";

export type { ColumnName, Row } from "./field-defs.js";

export interface FormatOptions {
  noColor: boolean;
}

export const formatRelativeTime = (timestamp: number): string => {
  const diff = Date.now() - timestamp * 1000;
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const weeks = Math.floor(days / 7);
  const months = Math.floor(days / 30);
  const years = Math.floor(days / 365);

  if (years > 0) return `${years} year${years > 1 ? "s" : ""} ago`;
  if (months > 0) return `${months} month${months > 1 ? "s" : ""} ago`;
  if (weeks > 0) return `${weeks} week${weeks > 1 ? "s" : ""} ago`;
  if (days > 0) return `${days} day${days > 1 ? "s" : ""} ago`;
  if (hours > 0) return `${hours} hour${hours > 1 ? "s" : ""} ago`;
  if (minutes > 0) return `${minutes} minute${minutes > 1 ? "s" : ""} ago`;
  return `${seconds} second${seconds !== 1 ? "s" : ""} ago`;
};

const ANSI_COLORS: Record<string, string> = {
  green: "\x1b[32m",
  yellow: "\x1b[33m",
  cyan: "\x1b[36m",
  magenta: "\x1b[35m",
  red: "\x1b[31m",
  dim: "\x1b[2m",
  reset: "\x1b[0m",
};

export const colorize = (text: string, color: string, noColor: boolean): string => {
  if (noColor) return text;
  const code = ANSI_COLORS[color] ?? "";
  return `${code}${text}${ANSI_COLORS.reset}`;
};

const formatCell = (row: Row, col: ColumnName, opts: FormatOptions): string => {
  const def = fieldDefs[col];
  if (!def) return "";
  const value = row[col];
  const text = def.format(value, row);
  const color = typeof def.color === "function" ? def.color(row) : def.color;
  if (color && !opts.noColor) {
    const code = ANSI_COLORS[color] ?? "";
    return `${code}${text}${ANSI_COLORS.reset}`;
  }
  return text;
};

export const formatTabular = (
  rows: Row[],
  columns: ColumnName[],
  opts: FormatOptions,
): string => {
  return rows
    .map((row) => columns.map((col) => formatCell(row, col, opts)).join("\t"))
    .join("\n");
};

export const formatJson = (rows: Row[]): string => {
  return JSON.stringify(rows, null, 2);
};
