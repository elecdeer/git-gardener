// Package format は list 系コマンドの出力（タブ区切り・JSON）を整形する。
package format

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Row は列名で値を取り出せる 1 行分のデータ。
type Row interface {
	// Field は列名に対応する値を返す。未知の列名なら nil。
	Field(name string) any
}

// fieldDef は列ごとの表示方法。
type fieldDef struct {
	// color は値に応じた色名を返す。nil または "" なら色なし。
	color func(v any) string
	// format は値を表示文字列に変換する。nil なら汎用の変換を使う。
	format func(v any) string
}

// fixedColor は値によらず同じ色を返す color 関数を作る。
func fixedColor(name string) func(any) string {
	return func(any) string { return name }
}

// checkMark は真偽値を ✓ / 空文字列で表す。
func checkMark(v any) string {
	if b, _ := v.(bool); b {
		return "✓"
	}
	return ""
}

var fieldDefs = map[string]fieldDef{
	"repo":             {},
	"host":             {},
	"owner":            {},
	"name":             {},
	"path":             {},
	"branch":           {color: fixedColor("cyan")},
	"worktreeCount":    {},
	"commitMessage":    {color: fixedColor("yellow")},
	"updateAt":         {},
	"updateAtRelative": {color: fixedColor("green")},
	"hash":             {color: fixedColor("yellow")},
	"upstream":         {color: fixedColor("magenta")},
	"ahead":            {color: fixedColor("green")},
	"behind":           {color: fixedColor("red")},
	"tracking":         {},
	"status": {color: func(v any) string {
		if v == "dirty" {
			return "red"
		}
		return "green"
	}},
	"isMain":    {format: checkMark},
	"isCurrent": {format: checkMark},
}

var ansiColors = map[string]string{
	"green":   "\x1b[32m",
	"yellow":  "\x1b[33m",
	"cyan":    "\x1b[36m",
	"magenta": "\x1b[35m",
	"red":     "\x1b[31m",
}

const ansiReset = "\x1b[0m"

// stringify は値を表示用の文字列に変換する。
func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(t)
	}
}

// formatCell は 1 セル分の文字列を返す。空文字列には色を付けない。
func formatCell(row Row, col string, noColor bool) string {
	def := fieldDefs[col]
	v := row.Field(col)
	text := stringify(v)
	if def.format != nil {
		text = def.format(v)
	}
	if noColor || text == "" || def.color == nil {
		return text
	}
	code, ok := ansiColors[def.color(v)]
	if !ok {
		return text
	}
	return code + text + ansiReset
}

// Tabular は rows を columns の順にタブ区切りで整形する。各行は改行で終わる。
func Tabular[R Row](rows []R, columns []string, noColor bool) string {
	var b strings.Builder
	for _, row := range rows {
		for i, col := range columns {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(formatCell(row, col, noColor))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// JSON は v をインデント付き JSON に整形する。末尾は改行で終わる。
func JSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// パスやコミットメッセージ中の < > & をエスケープせずそのまま出す。
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ParseColumns はカンマ区切りの列指定をパースする。valid に無い列名はエラーにする。
func ParseColumns(raw string, valid []string) ([]string, error) {
	var cols []string
	for c := range strings.SplitSeq(raw, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if !slices.Contains(valid, c) {
			return nil, fmt.Errorf("unknown column '%s' (available: %s)", c, strings.Join(valid, ", "))
		}
		cols = append(cols, c)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns specified (available: %s)", strings.Join(valid, ", "))
	}
	return cols, nil
}

// compareValues は同じ型の値同士を比較する。比較できない組み合わせは 0。
func compareValues(a, b any) int {
	switch va := a.(type) {
	case string:
		vb, _ := b.(string)
		// owner 名などの大文字小文字で並びが分断されないよう、大文字小文字を無視して比較する。
		if c := strings.Compare(strings.ToLower(va), strings.ToLower(vb)); c != 0 {
			return c
		}
		return strings.Compare(va, vb)
	case int:
		vb, _ := b.(int)
		return cmp.Compare(va, vb)
	case int64:
		vb, _ := b.(int64)
		return cmp.Compare(va, vb)
	case bool:
		vb, _ := b.(bool)
		switch {
		case va == vb:
			return 0
		case va:
			return 1
		default:
			return -1
		}
	}
	return 0
}

// SortRows は rows を field の降順（reverse なら昇順）に安定ソートする。
// field が valid に無ければエラーにする。
func SortRows[R Row](rows []R, field string, reverse bool, valid []string) error {
	if !slices.Contains(valid, field) {
		return fmt.Errorf("unknown sort field '%s' (available: %s)", field, strings.Join(valid, ", "))
	}
	slices.SortStableFunc(rows, func(a, b R) int {
		c := compareValues(b.Field(field), a.Field(field))
		if reverse {
			return -c
		}
		return c
	})
	return nil
}

// RelativeTime は unix 時刻 ts を now からの相対時刻（"3 hours ago" など）で表す。
func RelativeTime(ts int64, now time.Time) string {
	seconds := int64(now.Sub(time.Unix(ts, 0)) / time.Second)
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	plural := func(n int64, unit string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s ago", n, unit)
		}
		return fmt.Sprintf("%d %ss ago", n, unit)
	}

	switch {
	case days >= 365:
		return plural(days/365, "year")
	case days >= 30:
		return plural(days/30, "month")
	case days >= 7:
		return plural(days/7, "week")
	case days > 0:
		return plural(days, "day")
	case hours > 0:
		return plural(hours, "hour")
	case minutes > 0:
		return plural(minutes, "minute")
	}
	return plural(seconds, "second")
}
