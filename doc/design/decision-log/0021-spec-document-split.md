# 0021 Spec Document Split

- 状態: decided
- 作成日: 2026-06-03
- 最終更新日: 2026-06-03
- 関連: `../usage-flow.md`, `../output-format.md`, `../html-rendering.md`, `../cache.md`

## 背景

`doc/design/usage-flow.md` は How to Use 素案として始まったが、議論の蓄積に伴い 491 行まで肥大化した。内容には、利用者の操作の流れ(利用体験、Slack App 準備、token 注入、取得実行、channel 選択、エラー案内、実行例)だけでなく、出力ディレクトリ構造、保存 assets、取得範囲、サイズ制限、生成 HTML の見た目、中間ファイル `.cache/` の扱いといった「成果物の仕様」まで含まれていた。

「見栄え(HTML 表示仕様)」や「cache の扱い」は、利用手順(=操作の流れ)とは独立したトピックであり、単一ファイルに同居させると見通しが悪い。トピックに応じて文書を分割し、それぞれが対応する decision log と対応づくようにしたい。

## 候補

- A. 単一ファイルを維持する。
- B. 指摘された 2 トピック(HTML 表示仕様、cache)だけ抽出する 3 分割。
- C. 成果物軸で 4 分割する(usage-flow / output-format / html-rendering / cache)。
- D. トピック別に 6 分割する(usage-flow / fetch-scope / assets / output-format / html-rendering / cache)。

## 検討内容

A は変更がないが、肥大化と混在の問題が残る。

B は変更が最小で済むが、usage-flow.md に出力構造・assets・取得範囲が残り、依然として約 350 行と大きい。「操作の流れ」と「成果物の仕様」の混在が解消しきれない。

C は「利用者の操作の流れ」と「成果物の仕様(出力・見た目・cache)」という軸で分かれ、各文書が既存 decision log のクラスタ(出力: 0008 / 0013、assets: 0010 / 0014 / 0016 / 0017、取得範囲: 0011、HTML: 0012、cache: 0005)と対応づけやすい。ファイル数と相互リンク維持コストのバランスも実用的。

D は decision log とほぼ 1:1 で最も探しやすいが、ファイル数が増え、`fetch-scope` や `assets` が短い文書になりやすく、相互リンクの維持コストが上がる。現時点では過分割と判断。

## 決定

成果物軸で 4 分割する(候補 C)。

- `usage-flow.md`: 利用者の操作の流れ(利用体験、フェーズ 1〜3、処理対象の表示、channel 指定と選択、情報が足りない場合の案内、実行例、参考)。
- `output-format.md`: 出力ディレクトリ構造と label、保存する assets、取得範囲、添付ファイルのサイズ制限。
- `html-rendering.md`: 生成する `index.html` の表示仕様(見た目)。
- `cache.md`: 中間ファイル `.cache/` の役割、保持・削除・再利用の方針。

各新文書の冒頭に、他の分割文書と対応 decision log への相互リンクを置く。spec の正本はあくまで `doc/design/` 直下の各文書であり、decision log はその確定経緯を辿るための参考ログという建て付けを維持する。

既存 decision log の本文(背景・検討・決定・影響の記述)は当時の作業記録として保持し、文書分割に合わせて書き換えない。一方、冒頭の `関連` リンクは spec への navigation pointer なので、内容が移動したログでは移動先の新 spec 文書へ更新し、参考→正本の参照が現在の spec を指すようにする。

未決事項は各トピック文書に分散して記載しつつ、全体一覧は引き続き `decision-log/index.md` に集約する。

## 理由

「操作の流れ」と「成果物の仕様」を分けると、利用者は手順を、実装者は成果物仕様を、それぞれ必要な文書だけ読める。各文書が既存 decision log のトピッククラスタと対応するため、forward link で決定経緯へ辿りやすい。

既存 decision log の本文を書き換えないのは、decision log が「当時どう決めたか」の履歴であり、後からの文書再編で文面を改変すると履歴性が損なわれるためである。一方、冒頭の `関連` は spec への navigation pointer であり、移動先の spec 文書へ更新しても履歴性は損なわれない。本ログは分割という再編が行われた事実と新旧の対応を記録するが、spec の正本は `doc/design/` 直下の各文書であって本ログではない。

6 分割(D)を採らないのは、現時点の分量では `fetch-scope` / `assets` が小さく、ファイル数増による相互リンク維持コストが利点を上回ると判断したためである。

## 影響

- `doc/design/usage-flow.md` から出力形式・assets・取得範囲・HTML 表示仕様・サイズ制限・cache の各節を新文書へ移し、冒頭に分割文書への案内を追加する。
- `doc/design/output-format.md`、`doc/design/html-rendering.md`、`doc/design/cache.md` を新規作成する。
- `doc/design/README.md` の「主な文書」に新文書を追加する。
- `doc/design/decision-log/index.md` の「現在有効な主要方針」に本ログを追加する。
- `.github/copilot-instructions.md` の設計ドキュメントレビュー観点に新文書を追記する。
- 内容が移動した既存 decision log(0005 / 0008 / 0010 / 0011 / 0012 / 0013 / 0014 / 0016 / 0017)の `関連` リンクを移動先の新 spec 文書へ更新する。本文の影響範囲記述は当時の記録として据え置く。

## 後から見直す条件

- 文書がさらに増え、4 分割でも 1 文書が肥大化して見通しが悪化する。
- 逆に分割によって相互リンクや重複の維持コストが利点を上回る。
- 取得範囲や assets が独立文書に切り出すだけの分量・独立性を持つようになる。
