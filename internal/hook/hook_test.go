package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("フックを指定ディレクトリでシェルとして順に実行し、出力を out に流す", func(t *testing.T) {
		dir := t.TempDir()
		var out bytes.Buffer

		Run(t.Context(), []string{"echo one > log && pwd >> log", "echo two >> log; echo visible"}, dir, &out)

		log, err := os.ReadFile(filepath.Join(dir, "log"))
		if err != nil {
			t.Fatal(err)
		}
		realDir, _ := filepath.EvalSymlinks(dir)
		if got := string(log); got != "one\n"+realDir+"\ntwo\n" {
			t.Errorf("log = %q", got)
		}
		if out.String() != "visible\n" {
			t.Errorf("out = %q", out.String())
		}
	})

	t.Run("失敗したフックは警告を出し、後続のフックを続ける", func(t *testing.T) {
		dir := t.TempDir()
		var out bytes.Buffer

		Run(t.Context(), []string{"exit 3", "touch done"}, dir, &out)

		if !strings.Contains(out.String(), "gdn: warning: hook failed: exit 3") {
			t.Errorf("out = %q", out.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "done")); err != nil {
			t.Error("the following hook should still run")
		}
	})
}

func TestRunWithPath(t *testing.T) {
	t.Run("パスを末尾の引数として渡す（空白を含んでも 1 引数のまま）", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "with space")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := RunWithPath(t.Context(), "rm -r", target, dir, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("target should be removed: %v", err)
		}
	})

	t.Run("失敗したらエラーを返す", func(t *testing.T) {
		if err := RunWithPath(t.Context(), "false", "/x", t.TempDir(), &bytes.Buffer{}); err == nil {
			t.Error("expected an error")
		}
	})
}
