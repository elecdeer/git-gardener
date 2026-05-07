# git-gardener (`gdn`)

ghq と git worktree を統合したようなリポジトリ管理CLI。

- `gdn repo` — ghqライクなリポジトリのクローン・一覧管理
- `gdn wt` — git worktreeの作成・切り替え・削除・整理

インクリメンタルサーチはfzfなど外部ツールに委譲し、`gdn` はリスト出力と操作に専念する。

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
| `--no-color`          | ANSIカラーなしで出力            | false                   |
| `--json`              | JSON形式で出力                  | false                   |
| `--include-worktrees` | linked worktreeもリストに含める | false                   |

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

既存の `ghq.*` / `wt.*` 設定を検出して `gdn.*` に書き換える。

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
| `--include-branches` | worktreeに紐づいていないローカルブランチも含める | false                                        |

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
| `isMain`           | main worktreeかどうか（`✓` / 空）                    | なし |
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

指定ブランチのworktreeへ切り替える。worktreeが存在しない場合は作成する。worktreeの絶対パスをstdoutに出力する。

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

worktreeを新規作成する。既存ブランチ・worktreeがある場合はエラー。worktreeの絶対パスをstdoutに出力する。

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

worktreeとブランチを削除する。デフォルトブランチ（main等）は削除保護される。

```sh
gdn wt delete <branch> [options]
```

**オプション**

| オプション         | 内容                                                 |
| ------------------ | ---------------------------------------------------- |
| `--force`          | 未マージでも強制削除                                 |
| `-C, --dir <path>` | 対象リポジトリのパス（省略時はカレントディレクトリ） |

---

#### `gdn wt prune`

対象リポジトリのマージ済みブランチのworktreeとブランチを一括削除する。

```sh
gdn wt prune [options]
```

**オプション**

| オプション         | 内容                                                 |
| ------------------ | ---------------------------------------------------- |
| `--dry-run`        | 実際には削除せず対象を表示                           |
| `--yes` / `-y`     | 確認プロンプトをスキップ                             |
| `-C, --dir <path>` | 対象リポジトリのパス（省略時はカレントディレクトリ） |

---

#### `gdn wt migrate`

既存リポジトリのworktreeを `gdn.wtBasedir` 配下の正規化された構造へ移行する。bare リポジトリにも対応。

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

bare リポジトリ:

```
Before:
  ~/project.git/           bare repo
  ~/project.git/.wt/feature-branch/

After:
  ~/project/               新規作成（non-bare、デフォルトブランチをcheckout）
  ~/project.wt/feature-branch/   移動
```

**オプション**

| オプション         | 内容                                                 |
| ------------------ | ---------------------------------------------------- |
| `--dry-run`        | 実際には移動せず変更予定を表示                       |
| `--yes` / `-y`     | 確認プロンプトをスキップ                             |
| `-C, --dir <path>` | 対象リポジトリのパス（省略時はカレントディレクトリ） |

---

## シェル補完

`gdn complete <shell>` でシェル補完スクリプトを生成できる。

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

すべての設定は `gdn.*` 名前空間で管理する。グローバル設定（`~/.gitconfig`）またはリポジトリ設定（`.git/config`）に記述できる。

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
