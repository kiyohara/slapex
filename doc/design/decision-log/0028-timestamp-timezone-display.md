# 0028 時刻表示とタイムゾーン

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-13
- 関連: `doc/design/html-rendering.md`

## 背景

日時を「相対表現ではなく絶対時刻」で表示する方針(0012)は決まっていたが、どのタイムゾーンで、どの書式で表示するかが未確定だった。Slack API の時刻は epoch 由来の `ts` で渡されるため、表示時に必ずタイムゾーンの選択が発生する。

## 候補

- UTC 固定で表示する。
- workspace の default timezone を API から取得して使う。
- 実行環境の local timezone を使い、使用した timezone を HTML に明記する。

## 検討内容

- UTC 固定は曖昧さがないが、利用者が日常的に読む時刻と一致せず、archive を読む体験が悪い。
- workspace default timezone は Slack API 上で確実に取得できる field が限定的で、ユーザーごとの timezone 設定とも一致しない。
- 実行環境の local timezone は「export を実行した人がそのまま読む」という主用途と一致する。使用 timezone をヘッダに明記し、各時刻の `title` 属性に ISO 8601(UTC)を残せば、別 timezone の読者や機械処理にも正確な時刻を提供できる。

## 決定

- 表示は実行環境の local timezone、`YYYY-MM-DD HH:MM`(24 時間制)とする。
- `index.html` ヘッダに export 実行時刻と使用 timezone(UTC offset)を明記する。
- 各時刻要素の `title` 属性に ISO 8601(UTC)のフル時刻を入れる。
- timeline 上の日付の変わり目に date divider を表示する。
- dev / E2E の Docker Compose 実行では host の `TZ` を `dev` service に forward する。`--tz` のような専用 CLI option は導入しない。

## 理由

- 主用途(実行者自身による閲覧・保存)で最も自然に読め、かつ UTC のフル時刻を併記することで曖昧さを排除できるため。

## 影響

- HTML テンプレートは timezone 情報を受け取る。`.cache/metadata.json` の時刻は ISO 8601 UTC で記録する(`cache.md`)。
- CI 実行では runner の timezone(通常 UTC)が表示に使われる。明示したい場合は環境変数 `TZ` で制御できる(OS 標準の挙動に従い、専用 option は設けない)。
- Docker Compose 経由の dev / E2E 実行では、host 側で `TZ` が設定されていれば `compose.yaml` の `TZ: ${TZ:-}` によりコンテナへ引き継がれる。実行ごとに明示する場合は `docker compose run --rm -e TZ=Asia/Tokyo dev ...` のように指定する。

## 追記: コンテナ実行時の timezone

PoC 目視レビューで、Docker 経由の実行ではコンテナの default timezone(UTC)が時刻表示と出力ディレクトリ名に使われ、JST など host 側の期待とずれることが分かった。golang image には tzdata があり、`TZ` を渡せば JST 表示になることも確認済み。

この問題は dev / E2E のコンテナ実行に限られる。配布バイナリを host で直接実行する本来の利用形態では、既存決定どおり host の local timezone が使われる。そのため、対応は Compose で host の `TZ` を forward することに絞り、CLI option は追加しない。

## 後から見直す条件

- timezone を明示指定したい需要(例: チーム共有 archive で特定 timezone に固定)が出た場合。
