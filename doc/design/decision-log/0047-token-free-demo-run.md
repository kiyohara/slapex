# 0047 token 不要の demo 実行(--demo)

- 状態: decided
- 作成日: 2026-07-04
- 最終更新日: 2026-07-04
- 関連: `../cli-interface.md`、`../../guidelines/credential-scope-guidelines.md`、[0006-no-subcommands-initially.md](0006-no-subcommands-initially.md)、[0046-api-base-url-override.md](0046-api-base-url-override.md)、Issue #113、Issue #51

## 背景

Issue #51 で README のスクリーンショット・フロー図と同梱サンプル export(`doc/samples/`)を追加し、成果物の見た目は Slack App 準備前に確認できるようになった。しかし「自分の手元で slapex を実際に動かして試す」には依然として Slack App の作成と token 発行が必要で、試用のハードルが残っていた。Issue #113 はこの試用障壁をさらに下げるため、token 不要でサンプルデータから HTML export を生成できる実行経路を求めた。

既存資材として、`tools/gensample` に架空 fixture(ja / en の 2 シナリオ)と、それを配信する in-process fake Slack API server がある。Slack Web API の接続先を差し替える内部機構 `SLAPEX_API_BASE_URL`(0046)も既にある。

## 候補

1. 公開 option `--demo` を追加し、同梱 fixture を in-process の fake server 経由で通常の export pipeline に通す(自己完結)。
2. `slapex demo` のような subcommand を新設する。
3. 新しい公開 CLI は足さず、`tools/gensample -serve` + `SLAPEX_API_BASE_URL` の 2 コマンド手順を利用者向けに文書化する。

## 検討内容

- 候補 2 は subcommand 不採用の決定(0006)と衝突し、CLI 全体の形が変わる。demo のためだけに subcommand を導入するのは過剰。
- 候補 3 は Go toolchain / build が必要で、GitHub Releases のバイナリだけを持つ利用者には障壁が残り、試用障壁を下げる目的を十分に果たせない。加えて 2 コマンドを手で組み合わせる必要があり、単一コマンドで試せる体験にならない。
- 候補 1 は単一コマンド・token 不要・no subcommand で試用障壁が最も下がる。実現には `tools/gensample` の fixture / fake server / asset 生成を internal package(`internal/demo`)へ切り出し、`cmd/slapex` と `tools/gensample` の双方から import する必要がある。これにより release バイナリに架空 fixture(小さな SVG 群と最小 PDF)が同梱されるが、個人情報・実 token・実 workspace は含まない(#51 と同じ匿名化方針)。
- credential-scope の観点では、demo は内部専用の fake token を loopback の fake server にだけ送るため、実 token が実 Slack host 以外へ送られる導線を増やさない。接続先は CLI 内部で直接指定し、公開環境変数を経由しない。
- fixture は in-process 配信で実際の rate limit が無いため、通常実行の Slack API pacing(method ごとの待機)を demo で適用すると無意味な待ち時間になる。demo では pacing を省略して snappy に完了させる。
- 0046 の「後から見直す条件」にあった「#113 の demo 実行仕様が確定した時点で `SLAPEX_API_BASE_URL` の位置づけを判断する」への回答として、録画(token 入力プロンプトを見せる目的)は引き続き `SLAPEX_API_BASE_URL` を使い、利用者向けの token 不要経路は `--demo` として別に提供する、という棲み分けにする。

## 決定

候補 1 を採用する。公開 option `--demo` を追加する。指定時は次のように動作する。

- `SLACK_TOKEN` を要求せず、in-process の fake Slack API server を起動し、内部専用の fake token でその server にだけ接続して通常の export pipeline を実行する。
- fixture は `internal/demo` に置いた架空 ja / en シナリオを使い、locale(`LC_ALL` → `LC_MESSAGES` → `LANG` の順で最初の非空値)が `ja` で始まる場合は日本語、それ以外は英語を選ぶ。
- 対象 channel は fixture の単一 channel を non-interactive で自動解決する。positional な `[channel]` 引数は無視する。
- `--output` / `--no-color` / 取得範囲 option は通常実行と同じく尊重し、stdout の契約(成功時に出力ディレクトリ path を 1 行)も維持する。token 不要の案内は stderr に出す。
- 実際の rate limit が無いため Slack API pacing は省略する。

`tools/gensample` は `internal/demo` を import する形へ整理し、サンプル生成(`doc/samples/` 再生成)と `-serve`(録画用の長寿命 fake server)の挙動は維持する。

## 理由

- 単一コマンド・token 不要・no subcommand で、Slack App 準備前の試用障壁を最も下げられる。
- 既存の匿名化済み fixture / fake server を共有でき、サンプルと demo の見た目が一致する。
- credential-scope 方針(default deny)と整合する。demo は fake token を loopback にだけ送り、実 token の送信先を増やさない。
- 公開仕様の変更は option 追加 1 点に閉じ、token 受け渡し方針(0042: 環境変数のみ)や subcommand 不採用(0006)を崩さない。

## 影響

- 実装: `tools/gensample/{server,scenario_ja,scenario_en,assets}.go` を `internal/demo` へ移動し、`Scenario` / `Asset` / `ScenarioJA` / `ScenarioEN` / `FakeToken` / `NewServer` / `Handler` / `AllowAnyToken` / `WithAssetDelay` / `NoPacing` を公開。fixture を配信して export pipeline を回す共通ドライバ `demo.Export`(+ `demo.Options`)を追加し、`cmd/slapex` の `--demo`(`runDemo`)と `tools/gensample` のサンプル生成(`buildSample`)の双方をこれに集約。fake server / fake token / base URL / pacing 省略 / 単一 channel の non-interactive 解決という不変部分を 1 か所に閉じ、一方だけ直して他方が取り残される状態を避ける。`cmd/slapex/main.go` に `--demo` と `runDemo` / `demoScenario` / `demoPrefersJapanese` を追加。demo の取得窓は fake `conversations.history` を `oldest` で filtering(`filterSince`)して通常実行と揃える。
- テスト: `internal/demo` に fixture の end-to-end レンダリング(ja / en)、fake server の認証、placeholder 置換のテストを追加。`cmd/slapex` に `--demo` の parse / help 掲載 / locale 選択 / end-to-end の stdout 契約テストを追加。credential-scope の既存テスト(`SLAPEX_API_BASE_URL` の positive / negative)は維持。
- ドキュメント: `cli-interface.md` に `--demo` と「demo モード」節を追加。`README.md` の出力プレビュー・使い方に token 不要試用の案内を追加。
- 運用: `SLAPEX_API_BASE_URL`(0046)は録画用途としてそのまま残す。

## 後から見直す条件

- demo 用シナリオを増やす / 言語を切り替える公開 option の需要が出た場合(現状は locale 自動判定のみで公開 option を足していない)。
- 利用者向けに接続先差し替えを公開する需要が出て、`--demo` と `SLAPEX_API_BASE_URL` の棲み分けを再整理する必要が生じた場合(0046 と併せて再検討する)。
- 同梱 fixture のサイズやメンテコストが問題になり、fixture の埋め込み方式(生成コード / go:embed / 別配布)を見直す必要が生じた場合。
