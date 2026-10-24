package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/elecdeer/git-gardener/internal/format"
)

// jsonKeys は v を JSON にしたときのトップレベルのキーを出現順に返す。
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var keys []string
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

func TestListFields(t *testing.T) {
	tests := []struct {
		name   string
		row    format.Row
		fields []string
	}{
		{"repo list", repoRow{}, repoListFields},
		{"wt list", wtRow{}, wtListFields},
	}
	for _, tt := range tests {
		t.Run(tt.name+" の全フィールドを Field で取り出せる", func(t *testing.T) {
			for _, f := range tt.fields {
				if tt.row.Field(f) == nil {
					t.Errorf("Field(%q) returned nil", f)
				}
			}
		})
		t.Run(tt.name+" の JSON キーは列定義と同じ順に並ぶ", func(t *testing.T) {
			if got := jsonKeys(t, tt.row); !slices.Equal(got, tt.fields) {
				t.Errorf("json keys = %v, want %v", got, tt.fields)
			}
		})
	}
}

func TestDescribeConfigMigrations(t *testing.T) {
	plan := []configMigration{
		{oldKey: "wt.hook", newKey: "gdn.wtHook", values: []string{"a", "b"}},
		{oldKey: "wt.basedir", newKey: "gdn.wtBasedir", values: []string{"x"}, skip: true},
	}

	got := describeConfigMigrations(plan)

	want := []string{
		"Migrate wt.hook=a → gdn.wtHook=a",
		"Migrate wt.hook=b → gdn.wtHook=b",
		"Skip wt.basedir → gdn.wtBasedir: gdn.wtBasedir already set",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestConfirm(t *testing.T) {
	newCmd := func(stdin string) (*cobra.Command, *bytes.Buffer) {
		cmd := &cobra.Command{}
		var stderr bytes.Buffer
		cmd.SetIn(strings.NewReader(stdin))
		cmd.SetErr(&stderr)
		return cmd, &stderr
	}

	t.Run("yes ならプロンプトを出さない", func(t *testing.T) {
		cmd, stderr := newCmd("")
		if err := confirm(cmd, true, "Proceed?"); err != nil || stderr.Len() != 0 {
			t.Errorf("err = %v, stderr = %q", err, stderr.String())
		}
	})

	t.Run("y と答えれば続行する", func(t *testing.T) {
		cmd, _ := newCmd("y\n")
		if err := confirm(cmd, false, "Proceed?"); err != nil {
			t.Errorf("err = %v", err)
		}
	})

	t.Run("それ以外は ErrAborted", func(t *testing.T) {
		cmd, _ := newCmd("n\n")
		if err := confirm(cmd, false, "Proceed?"); !errors.Is(err, ErrAborted) {
			t.Errorf("err = %v", err)
		}
	})
}
