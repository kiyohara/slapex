# 0026 mrkdwn → HTML 変換とサニタイズ

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-11
- 関連: `doc/design/html-rendering.md`, `doc/design/slack-api-usage.md`

## 背景

`html-rendering.md` は見た目の方針(Slack default 風、JavaScript なし)を定めていたが、本文をどう HTML 化するか(mrkdwn の対応範囲、`blocks` の扱い、エスケープ方針)が未確定だった。生成 HTML はローカルブラウザで開かれるため、メッセージ本文経由の HTML / script 注入を仕様レベルで遮断しておく必要がある。

## 候補

- 本文ソース: (A) `text` フィールド(mrkdwn)を正とする。(B) `blocks`(rich_text)を構造的にレンダリングし、`text` は fallback。
- エスケープ: (A) Slack 由来テキストを全エスケープし、自前マークアップのみ生成。(B) 変換後 HTML をサニタイザーで後処理。

## 検討内容

- `blocks` の rich_text は表現力が高いが、要素種別が多く(rich_text_section / list / quote / preformatted、style の組み合わせ)、初期実装の負担が大きい。Slack は client 投稿でも `text` に mrkdwn 相当の fallback を生成するため、`text` ベースでも本文の大半は再現できる。
- mrkdwn の対応範囲は Slack 公式の記法(bold / italic / strike / code / pre / quote / link / mention / special mention / subteam / date token / emoji)に限定し、対応表として `html-rendering.md` に明記する。
- エスケープを「全テキストをエスケープしてから自前マークアップだけを組み立てる」方式 (A) にすると、サニタイザーという追加依存も、後処理漏れのリスクも避けられる。href は `http` / `https` のみ許可し、`javascript:` などの scheme を遮断する。
- 試作 slack_posts_dumper は HTML サニタイズに外部ライブラリ(bleach)を使っていたが、本リポジトリは「エスケープ済みテキスト + 自前マークアップ生成」を正とし、出力時サニタイズに依存しない方針とする(0001 のとおり試作の方針は暗黙採用しない)。

## 決定

- 本文は `text` フィールド(mrkdwn)を正とし、`html-rendering.md` の対応表に従って変換する。`blocks`(rich_text)の構造レンダリングは初期対象外とし、将来検討として未決事項に記録する。
- Slack 由来の全テキストは HTML エスケープ後に自前マークアップのみを生成する。リンク href は `http` / `https` のみ許可する。
- legacy attachment は URL preview 画像とタイトル・本文テキストの範囲で表示する。

## 理由

- `text` ベースは実装量と再現度のバランスが良く、PoC で機能充足性を測る範囲として適切。
- エスケープファーストの方針は、生成 HTML に JavaScript を含めない方針(0012)と合わせて、XSS 経路を構造的に塞げる。

## 影響

- `html-rendering.md` に変換対応表とサニタイズ方針を追記した。
- アーキテクチャ選定では、HTML テンプレートの自動エスケープ機構の有無を評価軸にする。
- `blocks` 完全対応を将来実装する場合、この decision log を superseded にして新しいログを作る。

## 後から見直す条件

- `text` fallback では再現できない投稿(リスト構造、複雑な書式)が実利用で問題になった場合。
- Slack が `text` fallback の生成を廃止・変更した場合。

## 追記: code 内の構文の扱い(2026-06-11)

PoC E2E の目視レビューで、code block 内にあった URL が HTML 上で表示されない問題が見つかった(経緯は PR #13 の記録)。Slack は inline code / code block の内部でも URL を `<URL>` 構文(auto-link)で格納するが、当初実装は code 内容を entity エスケープ済みの素テキストとみなして未処理のまま `<pre><code>` / `<code>` に出力していたため、生成 HTML に生の `<URL>` が残り、browser が未知タグとして解釈して URL が非表示になっていた。code 内の生 `<` `>` を素通しするため、エスケープ防御の穴でもあった。

次のとおり決定し、実装した。

- code span / code block の内容でも `<...>` 構文を解釈する。ただしリンクや mention のマークアップは生成せず、表示テキストへ展開する(`<URL>` は URL、`<URL|label>` は label、`<@U...>` / `<#C...|name>` / `<!here>` などの mention 系は通常本文と同じ表示テキスト `@表示名` / `#channel名` / `@here`)。
- 構文展開後に code 内容へ残る生の `<` `>` は HTML エスケープして出力する。利用者が入力した `<` `>` は Slack 側で `&lt;` `&gt;` 化済みのため、二重エスケープは起きない。

`html-rendering.md` の変換規則とエスケープ方針へ反映した。
