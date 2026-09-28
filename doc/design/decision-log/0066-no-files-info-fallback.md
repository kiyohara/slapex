# 0066 `files.info` による file 情報の補完を仕様から外す

- 状態: decided
- 作成日: 2026-09-28
- 最終更新日: 2026-09-28
- 関連: `doc/design/slack-api-usage.md`, `doc/design/output-format.md`, `doc/design/usage-flow.md`, `doc/design/html-rendering.md`, [0025-slack-api-usage-policy.md](0025-slack-api-usage-policy.md), [0017-uploaded-image-assets.md](0017-uploaded-image-assets.md)

## 背景

0025 は「file は message 内 file object を正とし、`files.info` は欠損時の補完のみ」と決め、`slack-api-usage.md` の「使用する API」も「file object に必要情報が欠けている場合だけ補完として呼ぶ」としていた。`output-format.md` の「保存する assets」の表も、ユーザーがアップロードした画像と画像以外の添付ファイルの取得元に `files.info` を挙げていた。一方、0025 の同じ「決定」が確定した使用 API の一覧に `files.info` は無く、実装も `files.info` を呼ばない。message の file object に download URL(`url_private_download` / `url_private`)が無いファイルは、補完せずに download URL の無いファイルとして扱い、`html-rendering.md` の「画像と添付ファイルの表示」の表のとおり表示する。Issue #289(PR #287 の作業中に、設計文書とコードを突き合わせて見つかった)。

補完が要る場面の手がかりとして、Slack の file object の文書は、Slack Connect channel にアップロードされたファイルについて、情報を省いた file object(`"file_access": "check_file_info"`)が届き、metadata を見るには追加の操作が要るとしている。

## 候補

- A: 仕様から外す。`files.info` を呼ばないことを仕様とし、`slack-api-usage.md` と `output-format.md` から補完の記述を外す。
- B: 補完を実装する。補完の条件(download URL が無い、`file_access` が `check_file_info` であるなど)と対象外のファイル(削除済み、Free plan の制限で非表示、外部サービス連携)を決め、`files.info` を呼ぶ。

## 検討内容

- Slack の Slack Connect の文書(「Additional check required to access file info (`check_file_info`)」の節。2026-09-28 に確認)は、情報を省いた file object が届くのは Events API / RTM API を listen する app であるとし、`conversations.history` / `conversations.replies` で message と file を取得する場合は完全な file object が返ると明記している。追加の API 呼び出しが要るのは、file の event が app へ push される場合だけとしている。file object の文書も、`check_file_info` を Events API / RTM API の payload の話として書き、詳細は Slack Connect の文書を参照させる。slapex は `conversations.history` / `conversations.replies` だけで message を取得し、Events API / RTM API を使わない。
- 補完の他の候補も、`files.info` で得られる情報が増えない。削除済み(`tombstone`)と Free plan の制限で非表示(`hidden_by_limit`)のファイルは、Slack がファイルの情報を伏せている。外部サービス連携のファイル(`is_external`)は、`url_private` / `url_private_download` が外部サービスを指すため download しない(0017 の 2026-09-27 の追記)。それ以外で download URL の無いファイルも、上記の文書のとおり message の file object は完全な file object であり、`files.info` が別の URL を返す根拠は無い。
- B は、文書上起きない場面のために、Web API の呼び出し(rate limit の pacing、再試行、失敗時の扱い、進捗表示)と、それを確かめる fake server の test を足すことになる。`files.info` の scope は既存の `files:read` で足り、rate limit は Tier 4 で、どちらも障害ではないが、効果を確かめられない。
- A は実装を変えない。設計文書と decision log を、実装と Slack の文書に揃えるだけである。

## 決定

A を採用する。

- `files.info` は呼ばない。file の情報は、`conversations.history` / `conversations.replies` の応答の message の `files` 配列だけから得る。
- file object に download URL などが無いファイルは補完せず、`slack-api-usage.md` の「file / asset の取得」と `html-rendering.md` の「画像と添付ファイルの表示」のとおり扱う(今の実装のまま)。
- 0025 の「`files.info` は欠損時の補完のみ」は、この決定で置き換える。0025 のそれ以外の決定は変えない。

## 理由

- Slack の文書が、slapex の取得経路(`conversations.history` / `conversations.replies`)では完全な file object が返ると明記しており、補完が要る場面が無い。
- 補完の対象になり得るファイル(削除済み、非表示、外部サービス連携、download URL の無いファイル)は、`files.info` を呼んでも得られる情報が増えない。
- 実装を変えずに、設計文書、decision log、実装を一致させられる(Issue #289 の完了条件)。

## 影響

- 文書: `slack-api-usage.md` の「使用する API」に `files.info` を呼ばないことと理由を書き、「参考」に Slack Connect の文書を足す。`output-format.md` の「保存する assets」の表の取得元から `files.info` を外し、表の下に `files.info` で補わないことを書く。`usage-flow.md` の「参考」から `files.info` の reference を外す。0025 に追記で参照を置き、`index.md` に行を足す。
- 実装、test、出力は変えない。使う Web API の method の一覧も変わらない。
- 必要な scope は変わらない。`files:read` は private file の download に引き続き要る(`doc/help/slack-app-setup.md`)。

## 後から見直す条件

- `conversations.history` / `conversations.replies` の応答に情報を省いた file object(`"file_access": "check_file_info"` など)が現れることが、実 workspace での確認や Slack の文書の変更で分かった場合。
- slapex が Events API / RTM API など、file の event の push を受ける経路を使うようになった場合。
- message の file object に無く、`files.info` でだけ得られる情報が export に要るようになった場合。
