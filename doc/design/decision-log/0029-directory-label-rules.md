# 0029 directory label の正規化規則

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-10
- 関連: `doc/design/output-format.md`, `doc/design/decision-log/0013-output-directory-labels.md`

## 背景

「label は ID そのものではなく人間が読みやすいものにする」方針(0013)は決まっていたが、具体的な正規化規則(文字の扱い、長さ、衝突時の動作、Unicode の可否)が未確定で、実装できる粒度になっていなかった。

## 候補

- ASCII のみに制限し、日本語などは ID へ fallback する。
- Unicode を保持し、filesystem で問題になる文字だけを置換する。
- 常に `<label>-<ID>` 形式にする。

## 検討内容

- 日本語 channel 名は一般的であり、ASCII 制限ではそれらがすべて ID になって可読性の目的(0013)を満たせない。
- 対象プラットフォームは macOS / Linux(0031)であり、Unicode ファイル名は通常問題にならない。問題になり得るのは separator・予約文字・制御文字・過剰な長さに限られる。NFC 正規化を仕様に含めると、macOS(HFS+/APFS の NFD 挙動)と Linux 間での見かけ上の不一致も避けやすい。
- 常時 ID 付与は衝突に最も強いが、通常ケースの可読性を下げる。衝突や空文字のときだけ ID を使う条件付き方式で十分。
- workspace は domain(`example.slack.com` の subdomain 部)が「人間が読める ASCII slug」としてそのまま使えるため、第一候補にする。

## 決定

`output-format.md` に次を確定した。

- `<workspace-label>`: workspace domain の subdomain 部 → workspace 名の正規化 → `team_id` の優先順。
- `<channel-label>`: NFC 正規化 → 禁止文字(`/ \ : * ? " < > |`、空白、制御文字)を `-` 置換 → `-` の圧縮と先頭末尾の除去 → 64 文字で切り詰め → 空なら channel ID。
- Unicode(日本語等)は保持する。同一出力 root 内での衝突時は ID 末尾 6 文字を suffix にする。
- 元の表示名・ID・使用 label の対応は `.cache/metadata.json` に記録する。

## 理由

- 可読性(0013)と filesystem 安全性を両立する最小の規則で、対象プラットフォームの実態に合っているため。

## 影響

- 実装は NFC 正規化と文字置換を行う小さな slug 関数を持つ。PoC で日本語 channel 名の出力を確認対象にする。
- Windows 対応を将来行う場合、予約名(`CON` など)や末尾 dot / space の追加処理が必要になる(0031)。

## 後から見直す条件

- Windows を対象に加える場合。
- zip 配布や Web 公開など、より厳しいファイル名制約を持つ共有経路が主用途になった場合。
