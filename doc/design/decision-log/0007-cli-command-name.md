# 0007 CLI Command Name

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../usage-flow.md`

## 背景

初期 CLI では subcommand を採用しない方針が決まり、root command 名を決める必要が出た。

## 候補

- `slack-posts-exporter`
- `slapex`
- `slarch`
- `slack2html`

## 検討内容

`slack-posts-exporter` は意味が明確だが、コマンドとしては長い。

`slapex` は `SLack Posts EXporter` の略であり、短く入力しやすい。発音しやすく、ブランド名として成立しやすい。検索時の競合も少ないと見込める。一方で、初見では Slack 関連ツールと分かりにくく、`slap` という語の連想が若干ある。

`slarch` は `Slack Archive` の略として用途を推測しやすく、語感も良い。ただし、既に同名の Slack log 保存ツールが GitHub 上に存在するため、OSS 化や配布時の名前衝突リスクがある。

`slack2html` は何をするツールか一目で分かり、README を読まなくても用途を理解しやすい。一方で、コマンドとしてはやや長く、ブランド性は低い。また、将来的に Markdown、PDF、JSON など HTML 以外の出力形式を追加した場合に名前との整合性が弱くなる。

## 決定

CLI command name は `slapex` とする。

基本形は次の通り。

```sh
slapex <channel-keyword> --output ./exports
```

## 理由

利用者が繰り返し実行する CLI として、短く入力しやすい名前を優先する。

また、将来的に HTML 以外の出力形式を追加しても、`slapex` は Slack posts exporter という意味を保てる。用途の分かりやすさだけなら `slack2html` も有力だが、初期段階では検索性、短さ、将来拡張との整合性を優先する。

## 影響

- `usage-flow.md` のコマンド例は `slapex` に統一する。
- 実装時の executable / package entry point は `slapex` を提供する。

## 後から見直す条件

- 既存の一般的な command name と衝突する。
- package manager や配布先で `slapex` 名が利用できない。
- ユーザーから略称が分かりにくいという feedback が出る。
- HTML 以外の出力形式を追加しない方針が固まり、用途の明快さを最優先する必要が出る。
