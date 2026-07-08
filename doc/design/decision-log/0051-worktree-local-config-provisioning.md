# 0051 worktree local config provisioning

- 状態: decided
- 作成日: 2026-07-08
- 最終更新日: 2026-07-08
- 関連: `.worktreeinclude`, `.agents/scripts/worktree-setup.sh`, `doc/guidelines/agent-configuration-management.md`, `.agents/mcp/github-op-integrated/README.md`, [0023-project-mcp-config.md](0023-project-mcp-config.md)

## 背景

0023 で、secret-free な MCP 起動定義(`.mcp.json` / `.cursor/mcp.json` / `.codex/config.toml`)を project 設定として git 管理し、`github-op-integrated` の secret reference は MCP 専用 `.config/github-op-integrated.conf` に置く方針を決めた。0023 はこのとき、`.worktreeinclude` や setup script で ignored local config を worktree へ配置する案を候補に挙げつつ、実装は将来へ据え置いた。

tracked な MCP 起動定義は worktree に入るが、gitignored な `.config/github-op-integrated.conf` は worktree へ自動配置されない。wrapper は自身の配置から project root を解決し、その直下の `.config/github-op-integrated.conf` を読むため、fresh worktree では config file が無く fail-loud で停止する。0023 が据え置いた worktree provisioning を実装するのが本ログの主題である。

## 候補

- 手動で各 worktree に local config を作る。
- `.gitignore` 対象を worktree へ機械的にコピーする。
- allowlist(`.worktreeinclude`)と setup script で、明示した local config だけを main worktree から worktree へコピーする。

## 検討内容

手動作成は最も単純だが、worktree ごとの設定漏れが起きやすい(0023 でも指摘済み)。`.gitignore` 全体のコピーは、build artifact や export 出力など無関係な ignored file を巻き込み、将来 raw secret を含む ignored file が増えたときに意図せず配布してしまうリスクがある。

allowlist 方式は、コピー対象を明示できるため「raw secret を含むファイルは載せない」という運用ルールと整合し、対象が 1 ファイル増減しても差分が読みやすい。project-wide `.env` を MCP tool へ渡す案は 0023 の security review で避ける方針になっているため、本ログでも踏襲する。

自動化については、worktree 作成時に setup script を自動起動する公式 hook は agent ごとに成熟度が異なる。先回りで hook を組むより、手順をドキュメント化し、手動実行(または各自の hook)で済ませる方が保守が軽い。

## 決定

- repo root に `.worktreeinclude`(commit)を置き、worktree へコピーしてよい gitignored local config を 1 行 1 path で allowlist する。raw secret を含むファイルは載せない。
- `.agents/scripts/worktree-setup.sh`(commit / 実行権限)が `.worktreeinclude` を読み、列挙ファイルが main worktree に存在する場合だけ現在の worktree へコピーする。既定は上書きせず、`--force` で上書きする。source は main worktree を自動判定し(`git --git-common-dir` の親)、`--source` で明示もできる。
- source に対象ファイルが無い場合は fail-soft とし、作成手順を stderr に案内する。MCP 起動時の最終 guard は wrapper 側の fail-loud を維持する。
- script はファイル内容を出力・log しない。file path と動作(コピー / スキップ / 未配置)だけを表示する。
- worktree 手順は `doc/guidelines/agent-configuration-management.md` の「worktree での ignored local config」に集約し、`.agents/mcp/github-op-integrated/README.md` に MCP 固有の補足を置く。自動化 hook は今回導入しない。

## 理由

allowlist + setup script は、0023 の「commit するのは secret-free な起動定義だけ」という境界を保ったまま、worktree の設定漏れを減らせる。コピー対象が明示されるため `.gitignore` 全体コピーより安全で、secret を含むファイルを配布しないという不変条件を運用ルールとして表現できる。

## 影響

- 新規: `.worktreeinclude`、`.agents/scripts/worktree-setup.sh`。
- 更新: `doc/guidelines/agent-configuration-management.md`(worktree local config 節を追加)、`.agents/mcp/github-op-integrated/README.md`(worktree 利用の補足)。
- worktree を作成したら、MCP を使う前に setup script を実行する運用になる。Codex のように worktree 初期化時に MCP を起動する場合は、setup script 実行後に MCP を再起動する。
- raw secret / 広い `.env` を MCP tool へ渡さない方針は維持する。

## 後から見直す条件

- Claude Code / Codex が worktree 作成時に setup script を自動実行できる公式 hook が定着し、手動手順が不要になった場合。
- allowlist 対象が増え、単純コピー以上の処理(権限調整、テンプレート展開、複数 source の解決)が必要になった場合。
- repository 固有 PAT ではなく複数 repository 横断の MCP 運用へ変わり、config の置き場自体を見直す場合(0023 の後から見直す条件と連動)。
