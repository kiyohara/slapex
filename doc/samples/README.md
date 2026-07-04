# doc/samples

このディレクトリには、slapex の成果物イメージを Slack App / token の準備なしで確認するための、生成済みサンプル export を置く(Issue #51)。

| path | 内容 |
|---|---|
| `ja/` | 日本語の架空 workspace「エージェントラボ」のサンプル export(`index.html` + `style.css` + `assets/`) |
| `en/` | 英語の架空 workspace「Agent Lab」のサンプル export(同上) |

リポジトリを clone またはダウンロードし、`doc/samples/ja/index.html`(または `en/index.html`)をブラウザで開くと、実際の出力と同じものをローカルで閲覧できる。

## データについて

- 舞台は、AI エージェント活用の情報を交換する架空のオープンなテックコミュニティ「エージェントラボ / Agent Lab」。学生や若手エンジニアのメンバーがイベント「エージェントナイト vol.3 / Agent Night vol. 3」を準備している、という設定。
- サンプルの workspace 名・ユーザー・メッセージ・画像・添付ファイルは、すべてこのサンプルのために作成した架空のもの。実在の workspace・コミュニティ・個人のデータは含まれない。
- メッセージ内容は表示パターンの網羅を意図している: システムメッセージ(参加 / トピック変更)、mrkdwn 装飾(太字 / 打ち消し / インラインコード / コードブロック / 引用 / リスト / リンク)、ユーザー / チャンネル / `@here` メンション、標準・カスタム絵文字、reaction、編集済みマーク、スレッド(参加者 4 名)、画像アップロード、URL unfurl、PDF 添付、bot 投稿、`/me` 投稿。削除済みメッセージ(tombstone)は表示文言が日本語のため `ja/` のみに含む。

## 再生成

サンプルは `tools/gensample` で生成する。架空データを in-process の fake Slack API server から供給し、実際の export パイプライン(`internal/export`)を通すため、生成物は常に現行レンダラーの出力と一致する。外部通信は発生しない。

```sh
docker compose run --rm -e TZ=Asia/Tokyo dev go run ./tools/gensample
```

日時は実行時刻から相対で決まるため、再生成すると本文中の日付・Export information の内容が更新される。

## README 用スクリーンショット

`assets/screenshots/` の `sample-*.png` は `tools/genscreenshot` で生成する。compose service `screenshot`(`tools/genscreenshot/Dockerfile`、headless Chromium + Noto CJK / emoji フォント)がこのサンプルの一時コピーを開き、撮影から切り出しまでをコンテナ内で完結させる。host のブラウザや macOS 固有の画像 tool には依存しない。

```sh
docker compose run --rm screenshot
```

撮影・検証の要点(詳細は `tools/genscreenshot/main.go` を正とする):

- viewport 幅 1100px・device scale 2x・scrollbar 非表示(`--hide-scrollbars`)で撮影し、幅 1600px へ縮小する。
- timeline 画像はページ先頭から最初の 1 日分(2 つ目の日付区切りの手前)まで。crop 境界は固定 pixel 値ではなく、ページに注入した測定スクリプトで block 位置を測って決める。
- thread 画像はスレッド親メッセージと、`<details class="thread-group" open>` に置換した一時コピーの展開スレッド部分。
- 生成後に画像 4 辺の pixel 検査を行い、右端の濃色 1px 縦線(Issue #132)のような境界 artifact が混入した場合は生成を失敗させる。

サンプルを再生成した場合はスクリーンショットも再生成する。フォントはコンテナ内の Noto フォントで決まるため、host 環境の違いで見た目が変わることはない。

## README 用デモ GIF

`assets/demo/slapex-demo-ja.gif` は、このサンプルの ja fixture を配信する fake Slack API server(`tools/gensample -serve`)に対して実際の slapex バイナリを実行し、VHS で録画したターミナル操作デモ(Issue #115)。`SLACK_TOKEN` 未設定での起動 → token 入力プロンプト(入力する token は架空値で、echo されないため画面には映らない)→ channel の対話選択 → 各フェーズの進捗表示 → 完了、までを収録している。slapex は内部環境変数 `SLAPEX_API_BASE_URL` で接続先を fixture server に差し替えており(`doc/design/decision-log/0046-api-base-url-override.md`)、録画はコンテナ内で完結し外部通信は発生しない。

再録画はリポジトリ root で次を実行する。

```sh
bash tools/demo/record.sh
```

dev container で linux 向けの `slapex` / `gensample` バイナリをビルドし、compose service `vhs`(`ghcr.io/charmbracelet/vhs` + CJK フォント。`tools/demo/Dockerfile`)で `tools/demo/demo-ja.tape` を再生して GIF を生成する。録画のシナリオ・待ち時間・画面サイズは tape 側で調整する。fixture の日時は実行時刻起点のため、再録画すると画面上の日付も更新される。
