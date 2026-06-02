# Git 操作ルール

この文書は、slack_posts_exporter リポジトリで AI agent が `git` コマンドを実行するときの共通ルールである。

## 1Password 連携が必要な操作

このリポジトリでは、次の `git` 操作で 1Password との連携が必要になる場合がある。

- 署名（commit / tag）を伴う操作:
  - `git commit`
  - merge commit を作る `git merge`
  - `git rebase --continue`
  - `git cherry-pick`
  - `git tag -s`
- GitHub の SSH remote への通信を伴う操作:
  - `git push`
  - `git fetch`
  - `git pull`
  - `git ls-remote`
  - `git clone`

  remote URL が HTTPS の場合、これらの操作は 1Password SSH agent 連携の対象外とする。

これらは 1Password SSH agent、署名（commit / tag）、または 1Password desktop app との連携に依存するため、AI agent の実行環境、sandbox、権限分離、TTY 設定などにより、socket 通信や承認プロンプトが阻害される場合がある。

各 AI agent は、現在の実行環境で 1Password 連携が阻害されることが事前に分かる場合、最初から制約のない実行環境で対象の `git` コマンドを実行する。

事前に判断できない場合は、通常の実行環境で試してよい。ただし、1Password SSH agent への接続失敗、署名失敗、承認プロンプト不達、SSH 認証失敗、socket 通信エラー、またはそれに類するエラーが出た場合は、同じ `git` コマンドを制約のない実行環境で再実行する。

## 通常の実行環境でよい操作

次のようなローカル参照・差分確認は、原則として通常の実行環境で実行してよい。

- `git status`
- `git diff`
- `git log`
- `git show`
- `git branch --show-current`
- `git rev-parse`

## 関連ルール

`gh pr create`、`gh pr view`、`gh run view` など、`git` コマンドではなく GitHub CLI(`gh`)を使う場合は `doc/guidelines/github-cli-guidelines.md` に従う。
