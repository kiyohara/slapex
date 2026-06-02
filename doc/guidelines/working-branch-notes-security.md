# Working Branch Notes 情報統制ルール

この文書は、`working-branch-notes/**/*.md` に書いてはいけない情報を定義する共通正本である。

`working-branch-notes/` の note は PR に含まれ、通常のリポジトリファイルとしてリモートへ送られる。そのため、作業メモであっても公開範囲が広がる前提で扱う。

## 基本方針

- 秘密情報、個人情報、顧客固有の非公開情報を書かない。
- 必要な文脈は、実値ではなくダミー値、プレースホルダ、変数名、抽象化した説明で残す。
- ログ、URL、設定値を貼る場合は、不要な識別子や認証情報を削ってから最小限だけ引用する。
- 判断に迷う情報は note に書かず、社内の適切な管理場所やマネージドシークレットを参照する。

## 書いてはいけないもの

- 認証情報、API key、access token、refresh token、cookie、session id、秘密鍵
- `.env`、credentials、CI secrets、クラウド設定、管理画面などに置かれる実値
- `BEGIN ... PRIVATE KEY` 形式の鍵、証明書、署名用秘密値
- password、secret、token などの名前に続く実値
- 個人情報、個人を直接識別できる情報、問い合わせ本文の生コピー
- 未公開の顧客名、顧客固有の設定、契約、運用、障害、利用状況の詳細
- 社内限定 URL、非公開ダッシュボード URL、認証情報付き URL
- 本番 URL に認証用 query parameter や署名付き parameter が付いたもの
- token、secret、cookie、session id、個人情報を含む可能性があるログ全文
- 長いランダム文字列など、資格情報や署名値に見える未確認の値

## 代わりに書くもの

- `API_TOKEN_PLACEHOLDER`、`<redacted>`、`example.com` などのダミー値
- `ENV_VAR_NAME` のような変数名
- 顧客名や個人名を出さない抽象的な説明
- エラー原因の理解に必要な stack trace の最小部分
- URL は host や path だけにし、query parameter は削除または redact したもの

## 編集時の確認

`working-branch-notes/**/*.md` を編集したら、push 前に少なくとも次を確認する。

- `password`、`secret`、`token`、`cookie`、`session`、`PRIVATE KEY` に続く実値がないか。
- 長いランダム文字列や署名値に見える文字列が残っていないか。
- URL に認証情報、署名、token、個人情報、顧客固有情報が含まれていないか。
- ログや問い合わせ文を必要以上に貼っていないか。

問題が見つかった場合は、実値を削除し、必要なら placeholder に置き換える。
