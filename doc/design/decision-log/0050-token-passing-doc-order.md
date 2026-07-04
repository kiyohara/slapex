# 0050 SLACK_TOKEN の渡し方の説明順位

- 状態: decided
- 作成日: 2026-07-04
- 最終更新日: 2026-07-04
- 関連: `../cli-interface.md`, [0042-default-user-token.md](0042-default-user-token.md), [0044-interactive-token-prompt.md](0044-interactive-token-prompt.md)

## 背景

PR #131(README 再構成・`doc/help/` 移設)の時点で、利用者向けドキュメント(`doc/help/token-injection.md`、`doc/help/usage.md`、quickstart、slack-app-setup)は 1Password CLI などの secret manager 連携を先頭・推奨として説明し、secret manager を使わない対話入力(0044 の token prompt)を「Secret manager を使わず一時的に渡す」として末尾に置いていた。

しかし利用者の大半は password manager / secret manager CLI を持っていないという認識があり、現状の説明順位では「secret manager が無いと本ツールを使えない」と誤解させるリスクがある。ツールの機能としては token prompt への都度コピー & ペーストだけで完走できる。

## 候補

1. 現状維持: secret manager 連携を先頭・推奨として説明する。
2. 説明順位を入れ替える: 対話入力(都度コピー & ペースト)を基本の方法として先頭に置き、環境変数への一時設定と secret manager 連携を補足的扱いにする。継続利用の推奨が secret manager 連携である点は変えない。
3. 推奨自体を対話入力へ変更する: secret manager 連携を単なる選択肢の 1 つに格下げする。

## 検討内容

- 候補 1 は、secret manager を持たない利用者(多数派の想定)が最初の説明でつまずく。token prompt(0044)はまさにこの層のために追加した機構であり、導線として活かせていない。
- 候補 2 は、初見の利用者が追加ツールなしで完走できることを最初に示しつつ、shell history に実値を残さないという安全性の説明も token prompt 経由なら簡潔になる。secret manager を持つ利用者への推奨(実値を都度扱わない・履歴に残さない)は補足位置でも伝わる。
- 候補 3 は、毎回の手動貼り付けは漏えい面(クリップボード・画面共有)と手間の双方で継続利用に向かず、secret manager を持つ利用者にとっての最適解を弱めてしまう。

## 決定

候補 2 を採用する。利用者向けドキュメントにおける `SLACK_TOKEN` の渡し方の説明順位を次のとおりとする。

1. **基本**: token 入力プロンプトへの都度コピー & ペースト(`SLACK_TOKEN` 未設定で実行し、prompt に貼り付ける)。
2. **補足**: shell 環境変数への一時設定(同じ token を複数コマンドで使い回す場合)。
3. **補足(継続利用の推奨)**: 1Password CLI などの secret manager からの実行時注入。
4. **CI 用途**: CI secrets からの注入(従来どおり)。

継続利用における推奨手段が secret manager 連携である点は変更しない。変更するのは説明の順位と扱い(基本 / 補足)のみである。

## 理由

- secret manager を持たない利用者が大半という前提では、追加ツールなしで完走できる方法を基本として最初に示すべきである。
- 「secret manager が無いと使えない」という誤解を防ぐことが、導入ハードルの低減(0041 の方向性)と整合する。
- token prompt(0044)は in-memory のみ・no-echo・非保存であり、基本の方法として安全性の説明が成り立つ。

## 影響

- `doc/help/token-injection.md`: 節順を「実行時に貼り付ける(基本)→ 環境変数への一時設定 → secret manager(継続利用の推奨)→ CI secrets」へ再構成する。
- `doc/help/usage.md`: 「実行の基本形」を token prompt 前提の最小コマンドで示し、`op run` 例は継続利用の推奨として補足に置く。
- `doc/help/quickstart.md`: 手順 2 の完了確認から「secret manager への保存」を必須に読める表現を外し、貼り付けだけで進められることを明示する。
- `doc/help/slack-app-setup.md`: token 保存の指示を「継続利用では保存を推奨」の表現に緩める。
- repo root `README.md`: Token の渡し方の紹介文の列挙順を合わせる。
- CLI の token prompt 文面(0044。継続利用には secret manager / CI secrets と案内)は変更しない。仕様(`cli-interface.md`)の変更もない。

## 後から見直す条件

- 利用者層の前提が変わり、secret manager 連携が一般的になった場合。
- token の誤共有・漏えい報告など、都度コピー & ペーストを基本とすることの安全性への懸念が顕在化した場合。
- token をファイル / OS keyring に保存する機構(0044 で見送り)を導入する場合。
