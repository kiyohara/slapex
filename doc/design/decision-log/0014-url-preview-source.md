# 0014 URL Preview Source

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../usage-flow.md`

## 背景

Slack message に含まれる URL preview 画像を保存対象にする場合、Slack API で取得できる unfurl / attachment 情報を使うか、ツール自身が URL にアクセスして Open Graph tag を取得するかを決める必要があった。

## 候補

- Slack API で取得できる unfurl / attachment 情報だけを使う。
- Slack API の unfurl 結果を優先し、存在しない場合はツール自身が URL にアクセスして Open Graph tag を取得する。
- Slack API の unfurl 結果を使わず、常にツール自身が Open Graph tag を取得する。

## 検討内容

Slack の見え方を再現する目的であれば、Slack が message に付与した unfurl / attachment 情報を使うのが自然である。

ツール自身が Open Graph tag を取得すると、Slack では preview が出ていなかった URL まで補完できる可能性がある。一方で、外部 URL への追加 network access、認証が必要な URL、robots / rate limit、取得失敗時の扱い、Slack 上の表示との差異が増える。

今回の目的は Slack で見えていた投稿内容をローカル HTML として保存することであり、Slack が表示していなかった preview まで補完することではない。

## 決定

URL preview 画像は、Slack API で取得できる unfurl / attachment 情報を利用する。

ツール自身が外部 URL にアクセスして Open Graph tag を取得する fallback は行わない。

Slack API の message に preview 情報が存在しない URL は、通常の link として表示し、preview 画像の保存対象にはしない。

## 理由

Slack での見栄え相当を再現することが目的であり、Slack が出していなかった preview まで補完すると、保存結果が Slack 上の表示と乖離する。

外部 URL への直接 fetch を行わないことで、実装と運用が単純になる。CI 実行時にも、Slack API 以外の任意の外部 URL へアクセスする必要がなくなる。

## 影響

- `usage-flow.md` の URL preview 画像の取得元は Slack unfurl / attachment 情報に限定する。
- `og:image` などの Open Graph tag をツール自身が直接取得する処理は初期実装に含めない。
- Slack API response に preview 画像 URL が含まれる場合だけ、その画像を保存対象にする。
- Slack API response に preview 情報がない URL は、通常 link として HTML に表示する。

## 後から見直す条件

- Slack unfurl が頻繁に欠落し、archive として不足が大きい。
- 利用者が Slack 上の見え方ではなく、URL preview の補完を明示的に求める。
- 外部 URL fetch を許可する network / security 方針が別途決まる。
