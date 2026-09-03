# 0055 asset の extension を download 内容から決める

- 状態: decided
- 作成日: 2026-09-03
- 最終更新日: 2026-09-03
- 関連: `../output-format.md`, `../cache.md`, `0016-asset-filenames.md`, `0052-content-hash-asset-filenames.md`

## 背景

0052 で asset のローカルファイル名を内容 hash(sha256)にしたとき、extension は「従来どおり `extensionFor`(元の表示ファイル名 → URL 拡張子 → content type)で決める」とした。URL の拡張子が内容と食い違うケースは想定していなかった。

v1.2.0 の実 workspace export(`--keep-cache` 付き)で、保存済み asset 56 件のうち 4 件が拡張子と内容の不一致になっていた。4 件はすべて gravatar 由来の avatar である。Slack の `users.info` が返す gravatar URL は path が常に `.jpg` で終わり、gravatar は `d=` に指定された Slack の default 画像(PNG)へ redirect する。URL 拡張子を Content-Type より優先していたため、内容が PNG でも `.jpg` として保存され、「ファイル名の extension」「manifest の `mimetype`」「ファイルの実体」が矛盾していた(Issue #183)。

内容 hash 命名の趣旨は「同じ実体なら同じ名前」である。同一内容の PNG が `.jpg` の URL と `.png` の URL の両方から来ると、内容 hash が同じでも extension が違うため別ファイルになる。0052 が定めた「同一内容・同一 extension は同一 path」という前提から見ても、URL の拡張子に引きずられるのは不自然である。

## 候補

- 現状維持(URL 拡張子を優先し、不一致は許容する)。
- Content-Type を URL 拡張子より優先する。
- download した内容を判別し、それを最優先する。
- 保存済みファイルを読み直して判別する。

## 検討内容

現状維持は表示には影響しないが、manifest の `mimetype` と `local_path` の extension が矛盾したまま残る。`--reuse-cache` は `local_path` を verbatim にコピーするため、矛盾は再利用のたびに引き継がれる。

Content-Type 優先は gravatar のケースを解消するが、依然として server の申告を信じる形になる。申告が欠落する host や、`application/octet-stream` を返す host では判断材料にならない。

内容の判別は実体を直接の根拠にできる。`http.DetectContentType` は先頭 512 byte の magic bytes で PNG / JPEG / GIF / WebP / BMP / ICO / PDF を判別でき、gravatar のように URL と実体が食い違う asset を確実に拾える。ただし SVG のように magic bytes を持たない形式は判別できないため、判別できない場合の fallback が必要になる。

保存済みファイルの読み直しは、0052 が「download 中に hash を計算して再読込しない」と定めた方針に反する。判別に必要なのは先頭 512 byte だけなので、既存の `io.MultiWriter`(一時ファイル + sha256)に小さな writer を足せば、再 download も再読込も不要で済む。

## 決定

asset の extension を、download した内容の判別結果から決める。

1. download した先頭 512 byte を `http.DetectContentType` で判別する。既知の型ならその extension を使う。`image/png` → `.png`、`image/jpeg` → `.jpg`、`image/gif` → `.gif`、`image/webp` → `.webp`、`image/bmp` → `.bmp`、`image/x-icon` / `image/vnd.microsoft.icon` → `.ico`、`application/pdf` → `.pdf`。
2. 判別できない場合は 0052 までの順序(元の表示ファイル名 → URL 拡張子 → Content-Type → `.bin`)へ fallback する。SVG はこの経路で従来どおり `.svg` になる。
3. Content-Type → extension の対応表に `image/x-icon` / `image/vnd.microsoft.icon` → `.ico`、`image/svg+xml` → `.svg`、`image/bmp` → `.bmp`、`application/pdf` → `.pdf` を追加する。URL に拡張子の無い favicon などが `.bin` になるのを避ける。

manifest の `mimetype` も同じ判断の並びに揃える。Slack の file metadata(upload / attachment で渡される `mimetype`)がある場合はそれを維持し、無い場合は 1 の判別結果、判別できなければ response の Content-Type を使う。

判別できた場合、extension と `mimetype` はどちらも同じ判別結果から決まるため一致する。判別できない場合は両者の根拠が分かれるため、一致は保証しない。例えば URL が `.jpg`、内容が判別不能、Content-Type が `image/svg+xml` の asset は `.jpg` + `image/svg+xml` になり得る。gravatar のように実体が判別できる形式で起きていた矛盾を解消することを目的とし、判別不能な形式まで一致させることは目的にしない。

## 理由

内容 hash 命名(0052)が「実体を基準に名前を決める」方針である以上、extension も実体を基準にする方が一貫する。URL や Content-Type の申告は host ごとに揺れるが、magic bytes は揺れない。

判別できない形式まで一致を追求すると、Content-Type を extension より優先するか、表示ファイル名を無視するかのどちらかが必要になる。前者は Slack の file metadata より server 申告を優先することになり、後者は添付ファイルの表示名と保存名が乖離する。いずれも実害の小さいケースのために既存の望ましい挙動を壊すため、fallback は 0052 までの順序のまま残す。

先頭 512 byte だけを保持する writer を既存の `io.MultiWriter` に足す形なら、0052 の「再 download / 再読込をしない」方針を保ったまま判別できる。

## 影響

- 実装: `internal/output` の `Assets.Save` が `headBuffer` で先頭 512 byte を保持し、`extensionFor` へ判別結果を渡す。`mimetypeFor` を追加し、manifest の `mimetype` を同じ並びで決める。再 download と一時ファイルの再読込は追加しない。
- テスト: gravatar 風 URL(path が `.jpg`)で PNG を返す asset が `.png` として保存され、manifest の `mimetype` が `image/png` になることを `internal/output` の unit test と `internal/export` の integration test で確認する。判別できない body では従来の順序が保たれることも既存 assert で確認する。
- 既存出力: 既に保存済みの archive や、`--reuse-cache` で引き継がれる旧 `.jpg` ファイル名は rename しない。HTML の表示はブラウザが内容で画像形式を判別するため変わらない。
- 同梱サンプル: `doc/samples/**` の asset は SVG と PDF で、SVG は判別対象外、PDF は URL 拡張子と実体が一致するため、出力に差分は出ない。
- 変更しないもの: 内容 hash 命名(sha256)、kind ディレクトリ構成、manifest を `source_url` 単位で記録すること、`--reuse-cache` の verbatim copy。cache schema の変更も無い。
- ドキュメント: `output-format.md` に extension の決定順序を、`cache.md` に `mimetype` の決め方を記載する。0052 の「extension は従来どおり」の部分は本ログで上書きし、0052 から本ログへ辿れるようにする。

## 後から見直す条件

- SVG のように magic bytes を持たない形式で、extension と `mimetype` の不一致が実害になる事例が出た場合。判別以外の根拠(Slack file metadata の優先度、Content-Type と表示ファイル名の優先順位)を見直す。
- Slack private file の Content-Type や file metadata を信頼できないケースが出た場合。本ログは Slack の file metadata を信頼する前提で `mimetype` を決めている。
- 判別対象に加えたい形式(magic bytes を持つが `http.DetectContentType` が扱わないもの)が出た場合。
