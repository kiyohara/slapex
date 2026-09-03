# 0052 asset ファイル名の内容 hash 化

- 状態: decided(extension の決め方は 0055 で download 内容からの判別優先へ変更)
- 作成日: 2026-07-08
- 最終更新日: 2026-09-03
- 関連: `../output-format.md`, `../cache.md`, `0016-asset-filenames.md`, `0030-cache-schema-and-reuse-validation.md`, `0055-asset-extension-from-content.md`

## 背景

0016 で asset のローカルファイル名を「元 URL の hash(md5)ベース」に決めた。その後 `tools/gensample` で同梱サンプル(`doc/samples/ja` / `doc/samples/en`)を再生成する運用が加わった。

サンプル生成は毎回 in-process fake server を立て、fixture の asset URL の `{{base}}` placeholder を実行ごとに変わる loopback URL へ置換してから実 export pipeline を通す(`internal/demo`)。このため asset の中身が同じでも URL の base 部分が実行ごとに変わり、URL hash 由来のファイル名も不必要に変わる。結果として、内容が変わっていないのにサンプルの asset ファイル名と HTML 参照が大量に差し替わり、再生成 diff が読みにくくなる。

0016 の「後から見直す条件」でも、署名付き query や実行ごとに変わる URL で同一実体が別 hash になる問題が大きい場合は hash 対象の正規化を検討する、としていた。今回はまさにその状況にあたる。

## 候補

- URL hash を維持し、hash 対象の URL を正規化する(base URL や署名 query を除外する)。
- ファイル名を download した「内容」の hash にする。
  - hash algorithm は md5 互換(32 桁)を保つか、sha256(64 桁)へ変えるか。

## 検討内容

URL 正規化は、正規化ルールを URL の種別ごとに保守し続ける必要がある。Slack private file、public unfurl、avatar、emoji など取得元が多様で、どの部分が「実体を表す安定部分」かは host ごとに異なり、規則が増えやすい。

内容 hash は、同じ bytes なら常に同じ名前になり、URL の揺れ(base、署名 query、CDN ホスト差)に一切依存しない。「内容が変われば名前も変わる / 変わらなければ名前も変わらない」という Issue #135 の期待に直接対応する。同一内容の重複保存も kind ディレクトリ単位で自然に 1 ファイルへ集約される。download 済みの一時ファイルへ書き込む bytes をそのまま hash すればよく、追加の download や再読込は不要。

algorithm は sha256 を選ぶ。用途は content addressing であり、md5 互換にする積極的な理由(既存ファイル名との一致)は方式変更で失われる。内容 hash 方針であることを名前から明確にする意味で sha256 が適切で、衝突耐性の面でも将来的に安心できる。ファイル名は 64 桁 hex + extension になるが、対象プラットフォームのファイル名長制限に十分収まる。

## 決定

asset のローカルファイル名を、元 URL の hash ではなく download した内容の sha256 hash にする。

- 保存名は `<content-sha256>.<ext>` を基本形とする。extension は従来どおり `extensionFor`(元の表示ファイル名 → URL 拡張子 → content type)で決める。
- 同一 kind ディレクトリ内で同一内容・同一 extension の asset は同一 path に集約する。異なる kind ディレクトリ間では directory が分かれるため、同一内容でも path は kind ごとに分かれる。
- `.cache/assets_manifest.json` は従来どおり `source_url` 単位で記録する。内容が同じ場合、複数の `source_url` が同一 `local_path` を指すことがある。
- 0016 の「種別ごとの分類ディレクトリ」「`assets/emoji/` への集約」「人間向け情報は manifest と HTML に持つ」は維持する。変えるのは hash の対象(URL → 内容)と algorithm(md5 → sha256)だけである。

## 理由

内容 hash はサンプル再生成 diff を実際に変わった asset だけに近づけられ、URL 正規化ルールの保守も不要になる。sha256 は content addressing の意図を名前から読み取りやすく、衝突耐性も高い。

## 影響

- 実装: `internal/output` の `Assets.Save` が download 中に sha256 を計算してファイル名にする(`io.MultiWriter` で一時ファイルへの書き込みと hash 計算を同時に行い、再読込しない)。既存 URL hash 由来のファイル名はすべて変わるため、同梱サンプル(`doc/samples/**`)を再生成して差し替える。
- `--reuse-cache`: `copyFromReuse` は旧 manifest の `local_path` を verbatim に使う挙動を維持する。同一 asset は内容が同じなので fresh download でも同じ content hash に解決され、内部整合は保たれる。旧 URL-hash 方式で作った cache を新実装で reuse すると、reuse 分だけ旧 URL-hash 名を保持する mixed layout になるが、各 manifest entry の `local_path` は実在ファイルを指し HTML 参照も壊れない(「前回保存物を verbatim に再利用する」という reuse の趣旨に沿う)。
- テスト: `internal/output/output_test.go` を内容 hash 前提へ更新し、同一内容・別 URL が同一 path へ集約されることを確認する unit test を追加した。`--reuse-cache` 系 integration test は byte 一致と request 数で検証しており algorithm 非依存。
- ドキュメント: `output-format.md` と `cache.md` を内容 hash 前提に更新する。0016 に追記して本ログへリンクする。

## 後から見直す条件

- 同一 kind ディレクトリ内で内容 hash が衝突する事態(現実的には sha256 でほぼ起きない)や、内容が同じでも別ファイルとして残したい要件が出た場合。
- 人間可読なファイル名の価値が衝突リスクを上回る場合(0016 の見直し条件と同じ)。

## 追記(2026-09-03)

本ログの「決定」にある「extension は従来どおり `extensionFor`(元の表示ファイル名 → URL 拡張子 → content type)で決める」という部分は、[0055-asset-extension-from-content.md](0055-asset-extension-from-content.md) で **download した内容の判別を最優先**する方式へ変更した。gravatar 由来の avatar のように、URL の path が `.jpg` でも実体が PNG である asset で、extension と manifest の `mimetype` とファイルの実体が矛盾していたためである(Issue #183)。

内容 hash 命名(sha256)、kind ディレクトリ構成、manifest を `source_url` 単位で記録すること、`--reuse-cache` の verbatim copy は本ログのまま維持する。判別も既存の `io.MultiWriter` に先頭 512 byte を保持する writer を足す形で行うため、本ログの「再 download / 再読込をしない」方針は保たれている。詳細は 0055 を参照する。
