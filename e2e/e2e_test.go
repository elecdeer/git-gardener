// Package e2e はビルドした gdn バイナリを実際の git リポジトリに対して実行するテスト。
package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// gdnBin は TestMain でビルドした gdn バイナリのパス。
var gdnBin string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// run は gdn をビルドしてからテストを実行し、終了コードを返す。
func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "gdn-e2e-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	gdnBin = filepath.Join(dir, "gdn")
	build := exec.Command("go", "build", "-o", gdnBin, "github.com/elecdeer/git-gardener/cmd/gdn")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to build gdn:", err)
		return 1
	}
	return m.Run()
}

// testEnv は 1 つのテスト用の隔離された git 環境。
type testEnv struct {
	t *testing.T
	// dir はテスト用の一時ディレクトリ（シンボリックリンク解決済み）。
	dir string
	// root は gdn.root。
	root string
	env  []string
}

// newTestEnv は HOME と git のグローバル設定を一時ディレクトリに隔離した環境を作る。
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	// macOS の /var → /private/var のようなシンボリックリンクを解決し、出力のパスと一致させる。
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "gdn-root")
	gitconfig := filepath.Join(dir, "gitconfig")
	content := fmt.Sprintf("[gdn]\n\troot = %s\n\tdefaultHost = github.com\n"+
		"[user]\n\tname = gdn test\n\temail = test@example.com\n"+
		"[init]\n\tdefaultBranch = main\n", root)
	if err := os.WriteFile(gitconfig, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	env := []string{"HOME=" + dir, "GIT_CONFIG_GLOBAL=" + gitconfig, "GIT_CONFIG_NOSYSTEM=1", "NO_COLOR="}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		// 実行環境の git 設定やリポジトリ指定がテストに漏れないようにする。
		if strings.HasPrefix(name, "GIT_") || name == "HOME" || name == "NO_COLOR" {
			continue
		}
		env = append(env, kv)
	}
	return &testEnv{t: t, dir: dir, root: root, env: env}
}

// result は gdn の実行結果。
type result struct {
	stdout string
	stderr string
	code   int
}

// gdnWith は stdin と追加の環境変数を指定して gdn を実行する。
func (e *testEnv) gdnWith(stdin string, extraEnv []string, args ...string) result {
	e.t.Helper()
	cmd := exec.Command(gdnBin, args...)
	cmd.Dir = e.dir
	cmd.Env = append(append([]string{}, e.env...), extraEnv...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			e.t.Fatalf("failed to run gdn: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// gdn は gdn を実行する。
func (e *testEnv) gdn(args ...string) result {
	e.t.Helper()
	return e.gdnWith("", nil, args...)
}

// gitMaybe は dir で git を実行し、stdout とエラーを返す。
func (e *testEnv) gitMaybe(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = e.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(string(out)), nil
}

// git は dir で git を実行し、失敗したらテストを中断する。
func (e *testEnv) git(dir string, args ...string) string {
	e.t.Helper()
	out, err := e.gitMaybe(dir, args...)
	if err != nil {
		e.t.Fatal(err)
	}
	return out
}

// initRepo は dir に 1 コミットだけ持つ main ブランチのリポジトリを作る。
func (e *testEnv) initRepo(dir, message string) {
	e.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	e.git(dir, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+filepath.Base(dir)), 0o644); err != nil {
		e.t.Fatal(err)
	}
	e.git(dir, "add", "README.md")
	e.git(dir, "commit", "-m", message)
}

var (
	hashPattern      = regexp.MustCompile(`\b[0-9a-f]{7}\b`)
	timePattern      = regexp.MustCompile(`\d+ (second|minute|hour|day)s? ago`)
	timestampPattern = regexp.MustCompile(`\d{10,13}`)
	trailingSpaces   = regexp.MustCompile(`[ \t]+\n`)
)

// normalize はテスト実行ごとに変わる値をプレースホルダーに置換し、前後の空白を除く。
func (e *testEnv) normalize(s string) string {
	s = strings.ReplaceAll(s, e.dir, "<testDir>")
	s = hashPattern.ReplaceAllString(s, "<hash>")
	s = timePattern.ReplaceAllString(s, "<time>")
	s = timestampPattern.ReplaceAllString(s, "<timestamp>")
	s = trailingSpaces.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}

// expectOutput は normalize した出力が want と一致することを確認する。
func (e *testEnv) expectOutput(name, got, want string) {
	e.t.Helper()
	if n := e.normalize(got); n != want {
		e.t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, n, want)
	}
}

// exists はパスが存在するかを返す。
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
