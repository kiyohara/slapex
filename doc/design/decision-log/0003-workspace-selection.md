# 0003 Workspace Selection

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../usage-flow.md`

## 背景

利用手順の素案では、Slack workspace を示す keyword と channel を示す keyword の両方を指定する案としていた。

しかし、利用者が bot token を実行時に渡す前提であれば、その token 自体が workspace を示すため、通常利用で workspace 指定が必要かを確認する必要が出た。

## 候補

- workspace keyword を必須オプションとして残す。
- workspace は bot token から解決し、利用者には channel だけ指定してもらう。
- Enterprise org-wide install を初期対象外とし、単一 workspace install の bot token だけを扱う。

## 検討内容

Slack Developer Docs では、Slack Web API に使う OAuth token は install 先 workspace のデータへアクセスするものと説明されている。

`auth.test` は token の認証と identity を確認する API であり、bot user token の成功レスポンスには workspace を示す `team_id`、workspace 名、workspace URL、bot ID が含まれる。

一方で Enterprise org-wide install では、token が複数 workspace の installation を表すケースがあり、一部 API は対象 workspace を示す `team_id` parameter を必要とする。単一 workspace installation の場合、`team_id` parameter は受け付けられるが無視されると説明されている。

今回の初期用途は各個人が自分用の Slack App を作成して利用する CLI であり、Enterprise org-wide install は対象外にしてよいと判断した。

## 決定

初期の利用手順では、workspace keyword を必須オプションにしない。

通常利用は単一 workspace install の bot token を前提とし、ツールは `SLACK_BOT_TOKEN` から `auth.test` などで workspace を解決する。利用者が指定する主対象は channel keyword とする。

Enterprise org-wide install や複数 workspace を 1 token で扱うケースは初期対象外とする。初期 CLI では `--team-id` option を提供しない。

org-wide install token であることが検出できる場合は、unsupported として扱い、単一 workspace install の bot token を使うよう案内する。

## 理由

単一 workspace install の bot token では、workspace は token から一意に分かる。必須の `--workspace` を置くと、利用者に重複指定を求めることになり、誤指定時の扱いも増える。

channel の曖昧性は残るため、利用者には channel 名または channel ID の指定を求め、複数候補が見つかった場合に再指定してもらう。

## 影響

- `usage-flow.md` のコマンド例から `--workspace` を外す。
- 出力パスの `<workspace>` は利用者指定ではなく、token から解決した workspace 名、workspace domain、または `team_id` をもとに決める。directory 名は人間が読みやすい label を優先し、詳細は `0013-output-directory-labels.md` に従う。利用者が指定する `--output` は出力 root とする。
- CI では workspace ごとに対応する `SLACK_BOT_TOKEN` を job に渡す。
- Enterprise org-wide install 対応は初期対象外とする。

## 後から見直す条件

- Enterprise Grid / org-wide install を正式対応範囲に含める必要が出る。
- 複数 workspace の export を 1 回の実行で扱う。
- 利用者が実行前に workspace 誤指定を防ぐための明示的な guard option を求める。

## 参考

- Slack Developer Docs: [Tokens & installation](https://docs.slack.dev/tools/python-slack-sdk/legacy/auth/)
- Slack Developer Docs: [`auth.test`](https://docs.slack.dev/reference/methods/auth.test/)
- Slack Developer Docs: [Enterprise Organizations](https://docs.slack.dev/enterprise/)
- Slack Developer Docs: [Developing apps for Enterprise orgs](https://docs.slack.dev/enterprise/developing-for-enterprise-orgs/)
