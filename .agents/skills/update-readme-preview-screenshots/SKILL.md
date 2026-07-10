---
name: update-readme-preview-screenshots
description: slapex の README 出力プレビューに映る HTML / CSS / assets、sample fixture、README の画像参照や caption、`tools/genscreenshot`、または `compose.yaml` の `screenshot` service を変更したときに使う。出力の見た目を変えた場合は先に `update-sample-exports` を実行し、その後に committed sample export から ja / en の preview screenshot 4 枚を Docker Compose で再生成・検証する。CLI テキスト出力、install script、開発者向け document、test、CI 設定だけの変更では使わない。
---

# update-readme-preview-screenshots

README の「出力プレビュー」で使う screenshot を、リポジトリに同梱した sample export の表示へ揃えるための skill。

## いつ使うか

次の変更で README 出力プレビューの見た目、掲載内容、または生成結果が変わる場合に使う。

- `internal/render/**`、`internal/render/templates/**`、`internal/export/**` の HTML / CSS、DOM 構造、表示内容。
- `internal/output/**` の asset path や保存仕様。
- `internal/demo/**`、`doc/samples/**` のうち screenshot に映る fixture、HTML、CSS、asset。
- repo root `README.md` の preview 画像参照、caption、画像を囲む HTML。
- `tools/genscreenshot/**`、`compose.yaml` の `screenshot` service の生成処理や実行環境。

CLI のテキスト出力、install script、開発者向け document、test、CI 設定だけの変更では使わない。変更が screenshot に影響するか判断できない場合は、README の画像参照と `tools/genscreenshot` が読む sample export までの経路を確認して決める。

## 事前確認

作業前に次を読む。

- `AGENTS.md`
- `doc/samples/README.md`
- `doc/guidelines/development-command-guidelines.md`

`git status --short` と対象差分を確認し、無関係な既存変更を上書きしない。

screenshot は `doc/samples/ja/` と `doc/samples/en/` に置かれた committed sample export から生成する。出力 HTML / CSS / assets の見た目、DOM 構造、asset path、fixture の表示内容を変えた場合は、先に `update-sample-exports` skill を実行し、sample export の実質差分を確定してからこの skill を実行する。

## 再生成

Docker が利用できることを `doc/guidelines/development-command-guidelines.md` の手順で確認してから、repo root で次を実行する。

```sh
docker compose run --rm screenshot
```

このコマンドは `doc/samples/<lang>/` の一時コピーを headless Chromium で開き、次の 4 画像を置き換える。

- `assets/screenshots/sample-timeline-ja.png`
- `assets/screenshots/sample-thread-ja.png`
- `assets/screenshots/sample-timeline-en.png`
- `assets/screenshots/sample-thread-en.png`

host のブラウザや画像編集 tool で代替生成しない。

## 生成後の確認

1. command output に 4 画像それぞれの path、画像サイズ、crop 範囲、`border check ok` が出ていることを確認する。各画像の幅は 1600px で、縦横の寸法が 0 でないことを確認する。
2. `git diff -- assets/screenshots` を確認し、画像の追加・削除や対象外 asset の変更がないこと、差分が対象実装・sample export・generator の変更から説明できることを確認する。生成条件に実質変更が無く差分も無い場合は、その結果を記録する。
3. 4 画像を個別に表示して、次を目視確認する。
   - timeline はページ先頭から最初の 1 日分までを含み、次の block を不自然に切っていない。
   - thread は親メッセージ、展開済み thread、続く code block / bot 投稿 / file attachment を想定どおり含む。
   - 文字、画像、reaction、caption 相当の表示が crop 端で欠けていない。
   - 右端を含む 4 辺に濃色の線、scrollbar、余計な border などの artifact がない。
   - PNG 自体に README 装飾用の枠線が焼き込まれていない。
4. repo root `README.md` の「出力プレビュー」を Markdown preview または GitHub 上で表示し、ja の timeline / thread 画像、caption、`<kbd>` 由来の枠線が期待どおり見えることを確認する。画像参照を変更した場合は、参照先が存在し broken image にならないことも確認する。
5. `git diff --check` を実行する。
6. 実行 command と結果、画像差分の有無、目視確認結果、未確認事項を PR description または working branch note に記録する。

生成物に説明できない差分、不自然な crop、欠落した内容、濃色 edge artifact が見つかった場合は commit せず、sample export、crop 計測、Chromium 実行環境のどこで差が生じたかを調べる。

## 責務の境界

- `update-sample-exports` は `doc/samples/ja/` と `doc/samples/en/` を実 export pipeline の出力へ揃える。
- この skill は確定した sample export から README preview screenshot 4 枚を生成・検証する。
- README ターミナルデモ GIF の再録画は `update-readme-demo-gif` skill の責務とする。

## やらないこと

- screenshot 更新のために sample fixture の題材や文言を無関係に変更する。
- PNG へ README 装飾用の枠線を焼き込む。
- host OS 上で `go run` や Chromium を直接実行する。
- README ターミナルデモ GIF をこの skill の一部として再録画する。
