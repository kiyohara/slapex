---
name: update-readme-demo-gif
description: slapex の CLI 出力、prompt、option、進捗・完了表示、demo mode、録画 fixture、README のターミナルデモ GIF の参照や caption、`tools/demo/**`、`tools/gensample/**`、`internal/demo/**`、または `compose.yaml` の `vhs` service を変更したときに、架空データだけを使って `assets/demo/slapex-demo-ja.gif` を再録画・検証する。出力 HTML / CSS、install script、help 文書、unit test、CI 設定だけの変更では使わない。
---

# update-readme-demo-gif

README の「ターミナルでの実行例」で使う GIF を、現行の CLI と架空 fixture を使った操作フローへ揃えるための skill。

## いつ使うか

次の変更でターミナルデモの操作フロー、表示内容、録画結果、または README での掲載意図が変わる場合に使う。

- `cmd/slapex/**` の CLI 出力、prompt、option 表示、demo mode、終了表示。
- `internal/ui/**` の進捗表示、色、plain / interactive 表示。
- `internal/export/**` の phase 名、summary、完了メッセージ、警告表示。
- `tools/demo/**` の録画 script、VHS tape、image、実行環境。
- `tools/gensample/**`、`internal/demo/**` のうち、録画 fixture、fake Slack API server、操作シナリオに影響する変更。
- `compose.yaml` の `vhs` service。
- repo root `README.md` のデモ GIF 参照、caption、alt text、掲載意図。

出力 HTML / CSS の見た目だけの変更は `update-sample-exports` と `update-readme-preview-screenshots` の責務とする。install script、help 文書、unit test、CI 設定、README のデモ GIF と無関係な文言だけの変更では使わない。影響するか判断できない場合は、変更箇所から `tools/demo/demo-ja.tape` が実行する CLI と fixture server までの経路を確認して決める。

## 事前確認

作業前に次を読む。

- `AGENTS.md`
- `doc/samples/README.md`
- `doc/guidelines/development-command-guidelines.md`
- `doc/guidelines/credential-scope-guidelines.md`

`git status --short` と対象差分を確認し、無関係な既存変更を上書きしない。

録画は `tools/gensample -serve` の架空 fixture と local fake Slack API server だけを使う。実 token、実 workspace、顧客固有データ、外部 Slack API は使わない。`tools/demo/demo-ja.tape` の token 入力値は架空値とし、echo されない状態を維持する。

## 再録画

Docker が利用できることを `doc/guidelines/development-command-guidelines.md` の手順で確認してから、repo root で次を実行する。

```sh
bash tools/demo/record.sh
```

この script は dev container で linux 向けの `slapex` / `gensample` を build し、compose service `vhs` で `tools/demo/demo-ja.tape` を再生して次の GIF を置き換える。

- `assets/demo/slapex-demo-ja.gif`

host OS に VHS や Go の開発環境を直接 install しない。実 token や実 workspace が必要に見える場合、外部 Slack へ通信する場合、または fixture server 以外へ認証情報を送信する場合は録画を止め、tape、環境変数、fake server の経路を確認する。

## 生成後の確認

1. `git diff --stat -- assets/demo/slapex-demo-ja.gif` と file 一覧を確認し、更新対象が GIF 1 ファイルだけであること、差分が対象の CLI、fixture、録画条件から説明できることを確認する。
2. GIF の先頭フレームと再生全体を表示し、次を目視確認する。
   - `export`、`unset`、fixture server 起動、環境変数設定などの準備コマンドが映っていない。
   - token の入力値、実 token、秘密情報、workspace 固有情報が映っていない。
   - token 入力 prompt、channel 選択、進捗表示、完了表示までが現行 CLI の文言と順序に一致する。
   - 最終の完了行まで収録され、途中で停止したり表示が切れたりしていない。
3. repo root `README.md` の「ターミナルでの実行例」を Markdown preview または GitHub 上で表示し、GIF が読み込まれ、指定幅で文字と操作内容を読めること、caption と表示内容が一致することを確認する。画像参照を変更した場合は broken image にならないことも確認する。
4. `git diff --check` を実行する。
5. 実行 command と結果、GIF 差分の有無、目視確認結果、未確認事項を PR description または working branch note に記録する。

準備コマンド、token、外部由来のデータ、説明できない表示差分、完了前の途切れが見つかった場合は commit せず、tape の `Hide` / `Show`、待ち時間、fixture、CLI 出力を確認して再録画する。

## README 用 media との境界

- `update-sample-exports` は `doc/samples/ja/` と `doc/samples/en/` を実 export pipeline の出力へ揃える。
- `update-readme-preview-screenshots` は確定した sample export から README preview screenshot 4 枚を生成・検証する。
- この skill は架空 fixture server に対する CLI 操作を録画し、README ターミナルデモ GIF を生成・検証する。

fixture の変更が sample export とターミナルデモの両方に影響する場合は、`update-sample-exports`、必要に応じて `update-readme-preview-screenshots`、この skill の順で各生成物を更新する。

## やらないこと

- 実 Slack workspace、実 token、顧客固有データを使って録画する。
- host OS に VHS、Go、録画用 dependency を直接 install する。
- CLI 出力デザインや録画 fixture の題材を、対象変更と無関係に作り直す。
- README preview screenshot や sample export を、この skill の一部として生成する。
- en 版 GIF を追加する。
