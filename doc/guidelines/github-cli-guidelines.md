# GitHub CLI 実行ルール

この文書は、slapex リポジトリで AI agent が GitHub CLI(`gh`)を実行するときの共通ルールである。

## GitHub MCP との関係

GitHub の PR / issue / レビューコメントなどの操作は、原則 `doc/guidelines/github-mcp-guidelines.md` に従って GitHub MCP Server を優先する。本ルールは次の場合に適用する。

- GitHub MCP Server が未設定、または当該ユーザー環境で利用できない。
- 対象の GitHub 操作が MCP 化対象の allowlist に含まれていない(merge、file push、release、workflow の実行・再実行・cancel・run log 削除、repository settings 変更などの高リスク write 操作はこちらに該当する)。CI の read(workflow / run / job / artifact の一覧と詳細、job log、check run、commit status)は MCP 側の allowlist にあるため、ここには該当しない。
- MCP 経由の実行が失敗し、`gh` で再試行する必要がある。

つまり本ルールは、GitHub MCP の fallback と、MCP 化対象外の操作に対する一次ルールである。

## 1Password op plugin 連携

`gh` コマンドを実行する前に、リポジトリ直下の `.op/` の有無と `op` コマンドの利用可否を確認する。

`.op/` が存在し、かつ `op` コマンドが実行可能な場合、`gh ...` を直接実行せず、次の形式を使う。shell alias に依存しない。

```sh
op plugin run -- gh ...
```

例:

```sh
op plugin run -- gh pr list
op plugin run -- gh pr view 123
op plugin run -- gh run view
```

`.op/` が存在しない、または `op` コマンドが利用できない場合は、通常の `gh ...` を使ってよい。

## 実行環境と 1Password desktop app 連携

`op plugin run -- gh ...` は 1Password desktop app との連携を必要とする。AI agent の実行環境、sandbox、権限分離、TTY 設定などにより、1Password desktop app との socket 通信や承認プロンプトが阻害される場合がある。

各 AI agent は、現在の実行環境で 1Password 連携が阻害されることが事前に分かる場合、最初から制約のない実行環境で `op plugin run -- gh ...` を実行する。

事前に判断できない場合は、通常の実行環境で試してよい。ただし、1Password desktop app への接続失敗、承認プロンプト不達、socket 通信エラー、またはそれに類するエラーが出た場合は、同じ `op plugin run -- gh ...` コマンドを制約のない実行環境で再実行する。

## 関連ルール

- `git commit`、`git tag -s`、GitHub の SSH remote を使う `git push` / `git fetch` など、GitHub CLI ではなく `git` コマンド自体を使う場合は `doc/guidelines/git-operation-guidelines.md` に従う。
- GitHub MCP Server 経由で実行可能な GitHub 操作は `doc/guidelines/github-mcp-guidelines.md` を優先する。
