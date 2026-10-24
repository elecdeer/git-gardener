package format

import (
	"slices"
	"testing"
	"time"
)

// testRow は map をそのまま Row として扱うテスト用の行。
type testRow map[string]any

// Field は列名に対応する値を返す。
func (r testRow) Field(name string) any { return r[name] }

var testFields = []string{"branch", "status", "isMain", "ahead", "updateAt", "path"}

func TestTabular(t *testing.T) {
	rows := []testRow{
		{"branch": "main", "status": "clean", "isMain": true, "ahead": 0, "path": "/p"},
		{"branch": "feat", "status": "dirty", "isMain": false, "ahead": 2, "path": ""},
	}

	t.Run("noColor なら色を付けず、真偽値は ✓ / 空で表す", func(t *testing.T) {
		got := Tabular(rows, []string{"branch", "status", "isMain", "ahead", "path"}, true)
		want := "main\tclean\t✓\t0\t/p\nfeat\tdirty\t\t2\t\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("色付きの列だけ ANSI カラーで囲み、status は値に応じて色を変える", func(t *testing.T) {
		got := Tabular(rows, []string{"branch", "status", "path"}, false)
		want := "\x1b[36mmain\x1b[0m\t\x1b[32mclean\x1b[0m\t/p\n" +
			"\x1b[36mfeat\x1b[0m\t\x1b[31mdirty\x1b[0m\t\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("行が無ければ何も出力しない", func(t *testing.T) {
		if got := Tabular([]testRow{}, []string{"branch"}, true); got != "" {
			t.Errorf("got %q", got)
		}
	})
}

func TestJSON(t *testing.T) {
	type row struct {
		Path string `json:"path"`
		N    int    `json:"n"`
	}

	got, err := JSON([]row{{Path: "<a&b>", N: 1}})

	want := "[\n  {\n    \"path\": \"<a&b>\",\n    \"n\": 1\n  }\n]\n"
	if err != nil || got != want {
		t.Errorf("got (%q, %v), want %q", got, err, want)
	}
}

func TestParseColumns(t *testing.T) {
	got, err := ParseColumns(" branch , path,,", testFields)
	if err != nil || !slices.Equal(got, []string{"branch", "path"}) {
		t.Errorf("got (%v, %v)", got, err)
	}

	if _, err := ParseColumns("branch,nope", testFields); err == nil {
		t.Error("unknown column should be an error")
	}
	if _, err := ParseColumns(",", testFields); err == nil {
		t.Error("empty columns should be an error")
	}
}

func TestSortRows(t *testing.T) {
	branches := func(rows []testRow) []string {
		var names []string
		for _, r := range rows {
			names = append(names, r["branch"].(string))
		}
		return names
	}
	newRows := func() []testRow {
		return []testRow{
			{"branch": "b", "updateAt": int64(100), "isMain": false},
			{"branch": "A", "updateAt": int64(300), "isMain": true},
			{"branch": "c", "updateAt": int64(200), "isMain": false},
		}
	}

	tests := []struct {
		name    string
		field   string
		reverse bool
		want    []string
	}{
		{"数値はデフォルトで降順", "updateAt", false, []string{"A", "c", "b"}},
		{"reverse で昇順", "updateAt", true, []string{"b", "c", "A"}},
		{"文字列は大文字小文字を無視して比較する", "branch", true, []string{"A", "b", "c"}},
		{"真偽値は true が先", "isMain", false, []string{"A", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := newRows()
			if err := SortRows(rows, tt.field, tt.reverse, testFields); err != nil {
				t.Fatal(err)
			}
			if got := branches(rows); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("未知のフィールドはエラー", func(t *testing.T) {
		if err := SortRows(newRows(), "nope", false, testFields); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestRelativeTime(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{0, "0 seconds ago"},
		{1 * time.Second, "1 second ago"},
		{59 * time.Second, "59 seconds ago"},
		{90 * time.Second, "1 minute ago"},
		{3 * time.Hour, "3 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{13 * 24 * time.Hour, "1 week ago"},
		{60 * 24 * time.Hour, "2 months ago"},
		{800 * 24 * time.Hour, "2 years ago"},
	}
	for _, tt := range tests {
		if got := RelativeTime(now.Add(-tt.ago).Unix(), now); got != tt.want {
			t.Errorf("RelativeTime(-%v) = %q, want %q", tt.ago, got, tt.want)
		}
	}
}
