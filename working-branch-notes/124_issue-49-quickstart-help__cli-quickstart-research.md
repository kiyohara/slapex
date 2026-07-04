# 作業ブランチメモ(補助): 有名 CLI ツールの quickstart 事例調査

- ブランチ: issue-49-quickstart-help
- PR: #124
- 最終更新: 2026-07-04

主 note: `124_issue-49-quickstart-help.md`

## 目的

`doc/help/quickstart.md` の改善のため、比較的有名な CLI ツールの quickstart / getting started ドキュメントに「どのような情報が」「どのように」記載されているかを調査し、slapex の quickstart へ反映できる示唆を整理する。

## 調査対象と各ツールの構成

公式ドキュメントを Web 検索・参照して確認した(2026-07 時点)。

### GitHub CLI(`gh`)

- 出典: docs.github.com の「GitHub CLI quickstart」
- 構成: About(1 段落)→ Prerequisites(install + `gh auth login`)→ Some useful commands(用途別の代表コマンド例)→ Getting help(`--help` の使い方)→ Customizing → Further reading
- 特徴: 認証は `gh auth login` の対話フローを正とし、プロンプトに従う前提で手順を短くしている。最後に「困ったら `--help`」という自己解決手段を必ず案内する。

### Stripe CLI

- 出典: docs.stripe.com の「Install the Stripe CLI」「CLI reference: login」
- 構成: Install(OS 別)→ **Get started without an account**(sandbox で口座なしで試せる)→ Log in(`stripe login`)→ CI 向け代替(`--interactive` / `--api-key`)
- 特徴: `stripe login` 実行時の**期待出力(pairing code の実例)をそのままコードブロックで掲載**し、利用者が画面と突き合わせられる。アカウントなしで試せる経路を install 直後に置く。CI などの分岐は「代替」として本筋から分離。

### Slack CLI / Slack app quickstart

- 出典: docs.slack.dev の「Quickstart」「Getting started with the Deno Slack SDK」
- 構成: Prerequisites → Install → `slack version` で確認 → `slack login`(**ターミナル出力の実例つき**)→ `slack auth list` で認証確認 → テンプレート app 作成 → 実行
- 特徴: **各ステップに「確認コマンド + 期待出力」がセット**で付く。認証成功の確認まで独立ステップとして扱う。

### 1Password CLI(`op`)

- 出典: 1password.dev の「Get started with 1Password CLI」
- 構成: Step 1: Install(OS / パッケージマネージャ別)→ Step 2: desktop app 連携を有効化 → Step 3: **任意のコマンドを 1 つ実行してサインイン**(`op vault list`)→ Next steps
- 特徴: ステップ数を 3 に絞り、最後のステップが「実際に使ってみる」= 検証を兼ねる。末尾に Next steps を必ず置く。

### Terraform

- 出典: developer.hashicorp.com の「Install Terraform」チュートリアル
- 構成: Install(OS 別 tab)→ **Verify the Installation**(`terraform -help`)→ トラブル時の注記 → 次のチュートリアルへの誘導
- 特徴: 「Verify the Installation」を独立見出しにする。quickstart 自体は薄く、続きはチュートリアルシリーズへ分割。

### Fly.io(`flyctl`)

- 出典: fly.io の「Quickstart: Launch your app」
- 構成: 番号付き 4 ステップのみ(Install → `fly auth signup` / `login` → `fly launch` → 必要なら `fly deploy`)→ Next steps(`fly status`、`fly apps open`)→ Grow and scale(発展リンク)
- 特徴: 本文が極端に短く、happy path 以外を一切書かない。完走後の確認(status / ブラウザで開く)を Next steps 側に置く。

### minikube

- 出典: minikube.sigs.k8s.io の「minikube start」
- 構成: 1 Installation(OS / arch セレクタ)→ 2 Start(`minikube start`)→ 3 Interact(`kubectl get po -A`)→ 4 Deploy(sample deployment)
- 特徴: 大きな番号ステップ + コピペ可能なコマンド 1 個ずつ。各ステップの直後に「結果を見るコマンド」が続く。

## 共通パターン(抽出)

1. **install 直後に検証コマンド**(`x version` / `x -help`)を置く。Terraform は独立見出し、Slack CLI は期待出力つき。
2. **期待出力をコードブロックで見せる**(Stripe / Slack CLI / minikube)。利用者が「自分の画面が正しい状態か」を照合できる。
3. **アカウント / 認証情報なしで試せる経路を先頭近くに置く**(Stripe の sandbox)。試用の心理的ハードルを下げる。
4. **認証は対話フローを正**とし、CI・自動化向けは「代替」として本筋から分離する(gh / Stripe)。
5. **ステップ数を 3〜5 に絞り、happy path しか書かない**(Fly.io / 1Password)。分岐や詳細はリンクへ逃がす。
6. **最後のステップは「実際に使う」= 検証を兼ねる**(1Password の `op vault list`、minikube の sample deploy)。
7. **末尾に Next steps セクション**を置き、発展的な使い方・詳細ドキュメントへ誘導する(Fly.io / 1Password / Terraform / gh)。
8. チェックボックス形式を採る例は少なく、番号ステップ + 確認コマンドが主流。ただし slapex は Issue #49 の要件(チェックリスト形式)があるため、チェックリストを維持しつつ「確認コマンド + 期待出力」を取り込むのが良い。

## slapex quickstart への反映判断

反映する:

- 完了確認に**期待出力の実例**を追加する(`slapex --version` → `slapex 1.1.2`、初回 export の完了 summary と stdout の出力 path)。出力例は `doc/design/usage-flow.md` の確定仕様の表示例と `--version` の実装(`slapex <version>`、release build は tag から `v` を除いた版)に合わせる。
- `--demo` を intro の言及から**独立した任意ステップ(ステップ 0)へ昇格**する(Stripe の「Get started without an account」相当)。token 準備前に成果物を確認できる経路を明示する。
- 末尾に**「次のステップ」セクション**を追加し、option 一覧・token の渡し方(継続利用)・制限事項 FAQ(#52 完成後)への誘導をまとめる(従来はステップ 4 の末尾 1 文だった)。

反映しない(理由つき):

- OS 別 tab UI: GitHub 上の素の Markdown では tab を再現できないため、現行の「Homebrew / script の 2 択 + 手動手順へのリンク」を維持する。
- ステップの大幅削減(Fly.io 型の 4 行構成): Slack App 作成という外部 UI 操作が必須のため、チェックリスト + 所要時間目安の現行構成の方が完走率に効くと判断。
- 認証確認の独立ステップ化(Slack CLI の `slack auth list` 相当): slapex には token 検証専用コマンドがなく、初回 export 自体が `auth.test` による検証を兼ねるため不要。

## 参照 URL

- https://docs.github.com/en/github-cli/github-cli/quickstart
- https://docs.stripe.com/stripe-cli/install / https://docs.stripe.com/cli/login
- https://docs.slack.dev/quickstart / https://docs.slack.dev/tools/deno-slack-sdk/guides/getting-started
- https://www.1password.dev/cli/get-started
- https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli
- https://fly.io/docs/getting-started/launch/
- https://minikube.sigs.k8s.io/docs/start/
