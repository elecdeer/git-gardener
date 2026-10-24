# git-gardener (`gdn`)

ghq と git worktree を統合したようなリポジトリ管理CLI。

- `gdn repo` — ghqライクなリポジトリのクローン・一覧管理
- `gdn wt` — git worktreeの作成・切り替え・削除・整理

インクリメンタルサーチはfzfなど外部ツールに委譲し、`gdn` はリスト出力と操作に専念する。

## インストール

[GitHub Releases](https://github.com/elecdeer/git-gardener/releases) から darwin / linux（amd64 / arm64）向けのバイナリを取得できる。

```sh
# mise
mise use -g github:elecdeer/git-gardener

# go install
go install github.com/elecdeer/git-gardener/cmd/gdn@latest
```

---

## コマンド一覧

### `gdn repo`

#### `gdn repo list`

管理下のリポジトリ一覧をstdoutに出力する。fzfなどに渡すことを想定。

```sh
gdn repo list [options]
```

**出力フォーマット（タブ区切り）**

`--column` で指定したフィールドを指定順にタブ区切りで出力する。色なしフィールドと色ありフィールドが混在してよい。

```
{col1}  {col2}  ...
```

**オプション**

| オプション            | 内容                            | デフォルト              |
| --------------------- | ------------------------------- | ----------------------- |
| `--column <fields>`   | 出力列をカンマ区切りで指定      | `repo,updateAtRelative` |
| `--sort <field>`      | ソートフィールド                | `updateAt` 降順         |
| `--reverse`           | ソートを昇順にする              | false                   |
| `--limit <n>`         | 上位n件のみ出力                 | 無制限                  |
| `--no-color`          | ANSIカラーなしで出力（`NO_COLOR` 環境変数でも無効化） | false |
| `--json`              | JSON形式で出力                  | false                   |
| `--include-worktrees` | linked worktreeもリストに含める | false                   |

`--include-worktrees` で追加される行の `repo` は `gdn.root` からの相対パス（例: `github.com/user/project.wt/feature`）になる。`gdn.root` 外にある worktree は絶対パス。

**`--column` で指定できるフィールド**

| フィールド         | 内容                                    | 色   |
| ------------------ | --------------------------------------- | ---- |
| `repo`             | リポジトリパス（host/user/project形式） | なし |
| `host`             | ホスト名のみ（`github.com`）            | なし |
| `owner`            | オーナー名のみ（`user`）                | なし |
| `name`             | リポジトリ名のみ（`project`）           | なし |
| `path`             | フル絶対パス                            | なし |
| `branch`           | HEADブランチ名                          | あり |
| `worktreeCount`    | linked worktree数                       | なし |
| `commitMessage`    | 最新コミットのサブジェクト              | あり |
| `updateAt`         | 最終コミットのunix timestamp            | なし |
| `updateAtRelative` | 最終コミットの相対時刻（"3 hours ago"） | あり |

**fzfとの組み合わせ例**

```sh
# col1=repo（キー・色なし）, col2=updateAtRelative（表示）
gdn repo list --column repo,updateAtRelative \
  | fzf --delimiter $'\t' --with-nth 2.. --nth 1 \
  | cut -f1
```

---

#### `gdn repo root`

`gdn.root` を解決した絶対パスを出力する。

```sh
gdn repo root
```

**使用例**

```sh
cd $(gdn repo root)
```

---

#### `gdn repo clone <url>`

リポジトリを管理構造でクローンし、`gdn.postCloneHook` を実行する。
構造はghqと同様で、`<gdn.root>/<host>/<user>/<project>` にクローンされる。すでに存在する場合はエラー。

```sh
gdn repo clone <url> [options]
```

URLは `github.com/user/project` や `user/project` など省略形も受け付ける。ホスト省略時は `gdn.defaultHost` を使用。

**オプション**

| オプション          | 内容                         | デフォルト |
| ------------------- | ---------------------------- | ---------- |
| `--shallow`         | shallow clone（`--depth 1`） | false      |
| `--branch <branch>` | 特定ブランチをクローン       | -          |
| `--submodules`      | submoduleを再帰的にクローン  | false      |

---

#### `gdn repo migrate [path]`

既存のローカルリポジトリをgdn管理のディレクトリ構造へ移行し、`gdn.postCloneHook` を実行する。`path` 省略時はカレントディレクトリを対象とする。

サブディレクトリや linked worktree から実行した場合もリポジトリ全体を移動し、移動後に `git worktree repair` で linked worktree からの参照を修復する。
リポジトリのローカル設定にある `wt.basedir` / `wt.hook` / `wt.deleteHook` / `wt.remover` を対応する `gdn.*` に移行する（移行先が設定済みならスキップ、旧キーは残す）。

```sh
gdn repo migrate [path] [options]
```

**オプション**

| オプション     | 内容                           |
| -------------- | ------------------------------ |
| `--dry-run`    | 実際には移動せず変更予定を表示 |
| `--yes` / `-y` | 確認プロンプトをスキップ       |

---

### `gdn wt`

#### `gdn wt list`

対象リポジトリのworktree一覧をstdoutに出力する。fzfなどに渡すことを想定。

```sh
gdn wt list [options]
```

**出力フォーマット（タブ区切り）**

`--column` で指定したフィールドを指定順にタブ区切りで出力する。

```
{col1}  {col2}  ...
```

**オプション**

| オプション           | 内容                                             | デフォルト                                   |
| -------------------- | ------------------------------------------------ | -------------------------------------------- |
| `--column <fields>`  | 出力列をカンマ区切りで指定                       | `branch,path,hash,updateAtRelative,tracking` |
| `--sort <field>`     | ソートフィールド                                 | `updateAt` 降順                              |
| `--reverse`          | ソートを昇順にする                               | false                                        |
| `--limit <n>`        | 上位n件のみ出力                                  | 無制限                                       |
| `--no-color`         | ANSIカラーなしで出力                             | false                                        |
| `--json`             | JSON形式で出力                                   | false                                        |
| `-C, --dir <path>`   | 対象リポジトリのパス                             | カレントディレクトリ                         |
| `--include-branches` | worktreeに紐づいていないローカルブランチも含める（`path` / `status` は空） | false |

**`--column` で指定できるフィールド**

| フィールド         | 内容                                                 | 色   |
| ------------------ | ---------------------------------------------------- | ---- |
| `branch`           | ブランチ名                                           | なし |
| `path`             | worktreeの絶対パス                                   | なし |
| `hash`             | 短縮コミットハッシュ                                 | あり |
| `upstream`         | upstreamブランチ名（`origin/feature`）               | あり |
| `ahead`            | upstreamより進んでいるコミット数                     | あり |
| `behind`           | upstreamより遅れているコミット数                     | あり |
| `tracking`         | upstream追跡状態（⇡/⇣）— `ahead`/`behind` の視覚表現 | あり |
| `commitMessage`    | 最新コミットのサブジェクト                           | あり |
| `status`           | working tree状態（`clean` / `dirty`）                | あり |
| `isMain`           | メインworktree（`git worktree list` の先頭）かどうか（`✓` / 空） | なし |
| `isCurrent`        | カレントworktreeかどうか（`✓` / 空）                 | なし |
| `updateAt`         | 最終コミットのunix timestamp                         | なし |
| `updateAtRelative` | 最終コミットの相対時刻                               | あり |

**fzfとの組み合わせ例**

```sh
# col1=branch（キー・色なし）, col2=path（キー・色なし）, col3以降=表示列
gdn wt list --column branch,path,hash,updateAtRelative,tracking \
  | fzf --delimiter $'\t' --with-nth 3.. --nth 1,2 \
  | cut -f1
```

---

#### `gdn wt root`

`gdn.wtBasedir` を解決した絶対パスを出力する。

```sh
gdn wt root [options]
```

**オプション**

| オプション         | 内容                                                 |
| ------------------ | ---------------------------------------------------- |
| `-C, --dir <path>` | 対象リポジトリのパス（省略時はカレントディレクトリ） |

**使用例**

```sh
cd $(gdn wt root)
```

---

#### `gdn wt switch <branch>`

指定ブランチのworktreeへ切り替える。worktreeが存在しない場合は `<gdn.wtBasedir>/<branch>` に作成し、`gdn.wtHook` を実行する。worktreeの絶対パスをstdoutに出力する。

```sh
gdn wt switch <branch> [options]
```

**オプション**

| オプション         | 内容                       | デフォルト           |
| ------------------ | -------------------------- | -------------------- |
| `--base <branch>`  | 新規作成時の分岐元ブランチ | カレントブランチ     |
| `-C, --dir <path>` | 対象リポジトリのパス       | カレントディレクトリ |

**シェル統合例**

```sh
# zsh: worktreeに切り替えてcdする
function gwt() {
  local path
  path=$(gdn wt switch "$1") && cd "$path"
}
```

---

#### `gdn wt create <branch>`

worktreeを `<gdn.wtBasedir>/<branch>` に新規作成し、`gdn.wtHook` を実行する。ブランチがローカルまたはリモートに既にあればそれを使い、無ければ `--base` から作成する。そのブランチのworktreeが既にある場合はエラー。worktreeの絶対パスをstdoutに出力する。

```sh
gdn wt create <branch> [options]
```

**オプション**

| オプション         | 内容                 | デフォルト           |
| ------------------ | -------------------- | -------------------- |
| `--base <branch>`  | 分岐元ブランチ       | カレントブランチ     |
| `-C, --dir <path>` | 対象リポジトリのパス | カレントディレクトリ |

---

#### `gdn wt delete <branch>`

`gdn.wtDeleteHook` を実行したうえでworktreeとブランチを削除する。デフォルトブランチ（main等）とメインworktreeは削除保護される。未マージのためブランチを削除できなかった場合はworktreeだけ削除して警告する。

```sh
gdn wt delete <branch> [options]
```

**オプション**

| オプション         | 内容                                                 |
| ------------------ | ---------------------------------------------------- |
| `--force`          | 未コミットの変更や未マージのコミットがあっても強制削除 |
| `-C, --dir <path>` | 対象リポジトリのパス（省略時はカレントディレクトリ） |

---

#### `gdn wt prune`

upstreamがリモートで削除済み（`[gone]`）のブランチについて、worktreeとローカルブランチを一括削除する。判定前に `git fetch --all --prune` を実行する。メインworktreeとデフォルトブランチは対象外。未コミットの変更があるなどで削除できないworktreeは警告してスキップする。

```sh
gdn wt prune [options]
```

**オプション**

| オプション         | 内容                                                 |
| ------------------ | ---------------------------------------------------- |
| `--dry-run`        | 実際には削除せず対象を表示                           |
| `--yes` / `-y`     | 確認プロンプトをスキップ                             |
| `--no-fetch`       | 判定前の `git fetch --all --prune` を行わない        |
| `-C, --dir <path>` | 対象リポジトリのパス（省略時はカレントディレクトリ） |

---

#### `gdn wt migrate`

既存リポジトリのlinked worktreeを `git worktree move` で `<gdn.wtBasedir>/<branch>` へ移行する。detached HEAD のworktreeは対象外。

```sh
gdn wt migrate [options]
```

**移行パターン**

通常リポジトリ（non-bare）:

```
Before:
  ~/project/               main worktree
  /tmp/feature-branch/     linked worktree（バラバラな場所）

After:
  ~/project/               そのまま
  ~/project.wt/feature-branch/   移動
```

bare リポジトリの移行は未対応。

**オプション**

| オプション         | 内容                                                 |
| ------------------ | ---------------------------------------------------- |
| `--dry-run`        | 実際には移動せず変更予定を表示                       |
| `--yes` / `-y`     | 確認プロンプトをスキップ                             |
| `-C, --dir <path>` | 対象リポジトリのパス（省略時はカレントディレクトリ） |

---

## シェル補完

`gdn complete <shell>`（`bash` / `zsh` / `fish` / `powershell`）でシェル補完スクリプトを生成できる。`gdn wt switch` / `gdn wt delete` ではブランチ名も補完される。

### zsh

```sh
mkdir -p ~/.zsh/completions
gdn complete zsh > ~/.zsh/completions/_gdn

# ~/.zshrc に以下を追加（初回のみ）
echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
echo 'autoload -U compinit && compinit' >> ~/.zshrc

source ~/.zshrc
```

### bash

```sh
mkdir -p ~/.local/share/bash-completion/completions
gdn complete bash > ~/.local/share/bash-completion/completions/gdn
source ~/.bashrc
```

### fish

```sh
mkdir -p ~/.config/fish/completions
gdn complete fish > ~/.config/fish/completions/gdn.fish
# 自動で読み込まれるため再起動不要
```

---

## 設定（git config）

すべての設定は `gdn.*` 名前空間で管理する。リポジトリ管理の設定はグローバル設定（`~/.gitconfig`）のみ、worktree管理の設定はグローバル設定とリポジトリ設定（`.git/config`）の両方から読む。

### リポジトリ管理

| キー                | 内容                                                         | デフォルト   |
| ------------------- | ------------------------------------------------------------ | ------------ |
| `gdn.root`          | リポジトリ配置ルートディレクトリ（複数設定はエラー）         | `~/gdn`      |
| `gdn.defaultHost`   | ホスト省略時のデフォルトホスト                               | `github.com` |
| `gdn.postCloneHook` | `repo clone` / `repo migrate` 後に実行するコマンド（複数可） | -            |

### worktree管理

| キー               | 内容                                       | デフォルト            |
| ------------------ | ------------------------------------------ | --------------------- |
| `gdn.wtBasedir`    | worktree配置先ディレクトリ                 | `../{gitroot}.wt`     |
| `gdn.wtHook`       | worktree作成後に実行するコマンド（複数可） | -                     |
| `gdn.wtDeleteHook` | worktree削除前に実行するコマンド（複数可） | -                     |
| `gdn.wtRemover`    | worktree削除に使うカスタムコマンド         | `git worktree remove` |

`{gitroot}` はリポジトリのルートディレクトリ名に展開される。

フックは `sh -c` で実行されるため、パイプやリダイレクトなどのシェル構文を使える。`postCloneHook` はクローン先、`wtHook` / `wtDeleteHook` は対象worktreeのディレクトリで実行される。フックの出力はstderrに流れ（stdoutはパス出力専用）、失敗しても警告を出して処理を続ける。

`gdn.wtRemover` を設定した場合は、worktreeのパスを末尾の引数として渡して実行し、その後 `git worktree prune` で git の管理情報を削除する。

### 設定例

```ini
# ~/.gitconfig
[gdn]
    root = ~/ghq
    defaultHost = github.com
    postCloneHook = git config gdn.wtBasedir "../{gitroot}.wt"
    wtHook = cp ../.env .env
    wtHook = pnpm install
    wtRemover = trash-put
```

---

## 開発

```sh
go test ./...                          # ユニットテスト + e2e テスト（e2e はバイナリをビルドして実行）
go run ./cmd/gdn --help
./test/run-e2e.sh                     # Docker の Linux 環境でテスト
```

リリースは `v*` タグの push で GoReleaser が実行される。
