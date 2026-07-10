---
name: update-sample-exports
description: slapex の出力 HTML / CSS / assets の見栄え、DOM 構造、asset 保存 path、demo fixture の表示内容、または `tools/gensample` の生成処理を変更したときに、架空データだけを使って `doc/samples/ja/` と `doc/samples/en/` の同梱サンプル export を再生成・検証する。README 文言、開発者向け doc、test、CI 設定だけの変更では使わない。
---

# update-sample-exports

slapex に同梱する生成済みサンプル export を、現行の export pipeline の出力へ揃えるための skill。

## いつ使うか

次のような変更がサンプル出力に反映される場合に使う。

- `internal/render/**`、`internal/render/templates/**` の HTML / CSS、DOM 構造、表示内容。
- `internal/export/**` の表示変換。
- `internal/output/**` の asset path や保存仕様。
- `internal/demo/**` の fixture のうち、export に表示される内容や asset。
- `tools/gensample/**` の生成対象、出力先、`demo.Export` の呼び出し方。

README の文言、開発者向け document、test、CI 設定だけの変更では使わない。変更がサンプル出力へ影響するか判断できない場合は、対象コードから generator / `demo.Export` までの経路を確認してから決める。

## 事前確認

作業前に次を読む。

- `AGENTS.md`
- `doc/guidelines/development-command-guidelines.md`
- `doc/samples/README.md`
- 認証情報や asset download の扱いを変更した場合は `doc/guidelines/credential-scope-guidelines.md`

作業ツリーに既存変更がある場合は、`git status --short` と対象差分を確認し、無関係な変更を上書きしない。

## 再生成

Docker が利用できることを `doc/guidelines/development-command-guidelines.md` の手順で確認してから、repo root で次を実行する。

```sh
docker compose run --rm -e TZ=Asia/Tokyo dev go run ./tools/gensample
```

このコマンドは `internal/demo` の架空データと local fake Slack API server だけを使い、実際の export pipeline を通して次を置き換える。

- `doc/samples/ja/`
- `doc/samples/en/`

外部通信、実 token、実 workspace のデータは使わない。実 token や実 workspace が必要に見える場合、または想定外の外部通信が発生した場合は実行を止め、fixture / generator の経路を確認する。

## 生成後の確認

1. `git diff -- doc/samples` を読み、変更が対象実装、fixture、generator から説明できることを確認する。実質差分だけを残し、相対日時 / Export information だけの差分は commit しない。実質差分か判断できない場合はユーザーに確認する。
2. `doc/samples/ja/index.html` と `doc/samples/en/index.html` が参照する `assets/` path を列挙し、参照先が各 sample directory 内に存在することを確認する。例えば次の command は欠落した path だけを出力する。

   ```sh
   for lang in ja en; do
     rg -o 'assets/[^" ]+' "doc/samples/$lang/index.html" |
       sort -u |
       while IFS= read -r asset_path; do
         test -f "doc/samples/$lang/$asset_path" || printf 'missing: %s/%s\n' "$lang" "$asset_path"
       done
   done
   ```

3. 変更した package に必要な test を Docker Compose 経由で実行する。対象が複数 package にまたがる場合は `internal/render`、`internal/export`、`internal/output`、`internal/demo` の関連 test を含める。
4. `git diff --check` を実行する。
5. 実行コマンドと結果、生成差分の要点、未確認事項を PR description または working branch note に記録する。

生成物に説明できない差分、欠落した asset、外部由来のデータが見つかった場合は commit せず、原因を調べる。

## README 用 media との境界

README 用 screenshot とターミナルデモ GIF の生成は、この skill の責務に含めない。

出力の見栄え、DOM 構造、fixture の表示内容を変えた場合は、サンプル export の更新後に `update-readme-preview-screenshots` skill を使う。CLI 操作や fixture シナリオの見え方を変えた場合は、デモ GIF の更新が必要になり得る。必要性を確認し、対応する別 skill / 手順を後続で使う。

## やらないこと

- README screenshot やデモ GIF をこの skill の一部として生成する。
- sample fixture の題材や文言を、対象変更と無関係に作り直す。
- host OS 上で `go run` や test を直接実行する。
- 実 token、実 workspace、顧客固有データを fixture の代わりに使う。
