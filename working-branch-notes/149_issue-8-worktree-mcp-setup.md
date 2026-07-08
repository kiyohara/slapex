# 作業ブランチメモ

- ブランチ: `issue-8-worktree-mcp-setup`
- PR: #149
- 最終更新: 2026-07-08

## 目的

GitHub Issue #8 の対応。worktree を使ったときでも project MCP tool
(`github-op-integrated`)が動く状態を作る。

tracked な MCP 起動定義(`.mcp.json` / `.cursor/mcp.json` / `.codex/config.toml`)は
worktree に入るが、gitignored な local config(`.config/github-op-integrated.conf`)は
worktree へ自動配置されない。fresh worktree では wrapper が config file 無しで
fail-loud に停止する。これを allowlist + setup script で補う。

## 現在の状況

- wrapper(`.agents/mcp/github-op-integrated/mcp-github-op-integrated.sh`)は
  自身の配置から `REPO_ROOT` を解決し、`$REPO_ROOT/.config/github-op-integrated.conf`
  を読む。worktree では worktree root 直下にこの config が無く fail-loud する、を確認済み。
- `.gitignore` は `.config/*.conf` と `.claude/worktrees/` を既に除外している。

## 決定事項

- repo root に `.worktreeinclude`(commit)を置き、worktree にコピーしてよい
  gitignored local config を allowlist する。raw secret を含むファイルは載せない。
- `.agents/scripts/worktree-setup.sh`(commit, 実行権限)が `.worktreeinclude` を読み、
  対象ファイルが main worktree に存在する場合だけ現在の worktree へコピーする。
  既定は上書きしない(`--force` で上書き)。ファイル内容は出力・log しない。
- source に対象ファイルが無い場合は作成手順を stderr に案内する(fail-soft)。
  MCP 起動時の最終 guard は wrapper 側の fail-loud を維持する。
- 自動化 hook は今回入れない。worktree 手順は
  `doc/guidelines/agent-configuration-management.md` に集約し、MCP README に補足を置く。
- 方針は decision log 0051 に記録する(0023 が据え置いた worktree provisioning の実装)。
- [PR #149 レビュー対応] `.worktreeinclude` は「repo root 相対 path」を安全境界にするため、
  絶対 path(`/` 始まり)や `..` segment を含む entry は path traversal リスクとして
  fail-loud(exit 2)で拒否する。source/dst 組み立て前に `case` で検証。
  missing>0 の fail-soft(exit 0)は 0051 の意図どおり維持(`--strict` は将来必要時)。

## 次にやること

- 実装した script の挙動検証(コピー / 既存スキップ / source 欠如時の案内)。
- `progress.md` の agent-env-01 行を更新。
- PR 作成(`Closes #8`)、note を PR 番号付きへ rename。

## 検証

throwaway git repo + 実 linked worktree(`.config/*.conf` を gitignore、config は
main の working dir にだけ存在、という実環境と同じ構成)で確認した。すべて期待どおり。

- `bash -n` 構文チェック: pass(shellcheck は未 install で skip)。
- TEST 1(fresh worktree, source に config あり): main worktree を自動判定し、
  対象ファイルだけを worktree へコピー。content 一致。allowlist に載せた
  未存在の `.config/github-op-integrated.conf` は「source に無い」+ 作成手順を
  stderr に案内、exit 0(fail-soft)。
- TEST 2(再実行): 既存ファイルは「既に存在するためスキップ」。上書きしない。
- TEST 3(`--force`): source を変更後 `--force` で上書き反映。`--force` 無しでは非上書き。
- TEST 4(main worktree 上で実行 = source==dest): `-ef` 判定で「既に配置済み」、自己コピー無し。
- TEST 5: 不正 arg は exit 2、`--help` は exit 0。
- セキュリティ: ダミー config に埋めた marker 文字列が script 出力に一切現れないことを確認
  (file 内容を出力・log しない)。
- [PR #149 レビュー対応] path traversal: `../escaped.conf` / `.config/../../x.conf` /
  絶対 path `/tmp/x.conf` / `foo/..` はすべて exit 2 で拒否し、worktree 外に何も書かない。
  正常 entry(`.config/dummy.conf`)は従来どおりコピー。filename 内の `..`
  (`.config/..hidden.conf`、segment ではない)は誤検知せず許可、を確認。

環境依存で本ブランチ内フル検証していない点:

- worktree での MCP `tools/list` と read-only GitHub API 呼び出し成功(Docker + 1Password +
  実 PAT が必要)。script の機構は上記 temp 検証で担保。wrapper の fail-loud 案内は既存実装で、
  worktree でも `$REPO_ROOT/.config/github-op-integrated.conf` 不在時に発火する(コード確認済み)。
  なお main worktree では MCP tool(`github-op-integrated`)が本セッションで実際に稼働している。

## リスク・ブロッカー

- MCP `tools/list` + read-only API 呼び出しの完全検証は Docker + 1Password +
  実 PAT が必要で、環境依存。script 機構は temp dir 検証で担保し、実 API 検証は手動確認扱い。
- secret 実値を diff / log / note / PR に出さない(ダミー値だけで検証する)。

## セッションログ

- 2026-07-08: Issue #8 着手。context 確認、branch 作成、方針確定、実装。
  `.worktreeinclude` / `worktree-setup.sh` / decision log 0051 / guideline・MCP README 更新。
  実 worktree で挙動検証(TEST 1-5 + セキュリティ)を pass。PR #149 作成。
- 2026-07-08: PR #149 レビュー対応。Codex [must] 指摘の path traversal を
  `worktree-setup.sh` に検証追加(絶対 path / `..` segment を exit 2 で拒否)。
  `.worktreeinclude` の contract もコメントに明記。traversal / regression / 誤検知を再検証。
