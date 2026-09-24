# 0060 model の識別子の記載範囲と review metadata の Model

- 状態: decided
- 作成日: 2026-09-24
- 最終更新日: 2026-09-24
- 関連: `doc/guidelines/pull-request-guidelines.md`, `.agents/skills/review-pull-request/SKILL.md`, [0057-pr-tool-name-restriction-scope.md](0057-pr-tool-name-restriction-scope.md), [0059-issue-review-cycle-orchestration.md](0059-issue-review-cycle-orchestration.md), [0061-review-metadata-in-cloud-session.md](0061-review-metadata-in-cloud-session.md)

## 背景

slapex の正本は、model の識別子をどこに書いてよいかを決めていなかった。

- 0057 と `doc/guidelines/pull-request-guidelines.md` は tool 名の扱いだけを定め、model の識別子に触れていない。
- `review-pull-request` の canonical metadata は、`Model` に実行環境で確認できる値を書き、確認できない場合だけ `unknown` とする。実行環境の指示と食い違う場合にどちらに従うかは書いていない。

kiyohara/bizdate では、cloud session で review cycle を回した際に、`Model` が投稿によって `unknown` と識別子に分かれた(bizdate の Issue #68)。harness が agent に与える既定の指示(model の識別子を commit message、PR の title と本文、code comment など repository に push する成果物に含めない)を、orchestrator が PR のコメントにも当てはめたためである。bizdate は PR #72(bizdate の decision log 0023)で記載範囲を決めた。

slapex でも Issue #227 で `drive-issue-to-reviewed-pr` を追加し、cloud session で review cycle を回す。2026-09-24 の project の thread の session では、実行環境の指示が model の識別子を「チャットの返答以外に書かない」よう求めており、同じ判断が要る。ユーザーは #227 の作業中に、bizdate PR #78(0061)とその前提の PR #72 に相当する変更を #227 に含めると決めた(2026-09-24)。

## 候補

記載してよい範囲:

- A: tool 名と同じく、禁止を PR title に限る。それ以外は制限せず、書き手の裁量に任せる。
- B: PR title に加え、code と文書の本文でも禁止する。記録は trailer と PR 側に限る。
- C: harness の既定の指示に揃え、repository に残るもの(commit message、PR の title と本文、code と文書)では禁止し、PR と review のコメントだけ許可する。

review metadata の `Model`:

- D: 実行環境で確認した識別子を書く。実行環境の指示を理由に `unknown` にしない。
- E: 実行環境の指示が識別子を禁じる場合は `unknown` とする。
- F: D を基本とし、実行環境の指示が PR と review のコメントへの記載まで禁じる場合(または対象外と確認できない場合)に限って `unknown` とし、その旨を投稿に残す。

## 検討内容

- A: 0057 が tool 名について取った理由(禁止は title の一覧性を守れば足りる、各 tool の既定動作と恒常的に衝突させない)が model の識別子にもそのまま当てはまる。model の識別子は tool 名と同じく title の一覧性を損なうため、title では禁止する。
- B: code と文書に識別子を書いて困った事例はまだ無い。`doc/guidelines/agent-configuration-management.md` の「ルールをシンプルに保つ」(先回りで仮想シナリオに備えない)に反する。
- C: 特定の harness の既定の指示を repository のルールへ写すことになる。harness ごとに指示が違い、変わり得るため、repository のルールとして追従しきれない。bizdate の harness の指示と、slapex の project の thread の指示も文面が異なっていた。
- A の範囲では、任意の記載を harness の指示に従って省くことは repository のルールと衝突しない。衝突が起き得るのは、記載を必須とする review metadata の `Model` だけである。
- D: `Model` は review cycle を後から辿るための記録であり、同じ cycle の投稿で値が揃わないと記録として使えない。canonical metadata は PR と review のコメントに置かれ、repository に push される成果物ではない。ただし D だけでは、実行環境の指示がコメントへの記載まで実際に禁じる場合も「書け」と読める。repository の guideline は agent の上位の指示を上書きできないため、その環境では agent が両方を守れない。slapex の project の thread の指示(チャットの返答以外に書かない)はこれに当たり得る。
- E: 同じ Agent 種別でも、投稿者(orchestrator と subagent)の解釈で値が変わる。記録目的を満たさない。
- F: 指示の対象が repository に push する成果物だけでコメントを含まない環境では、D と同じ結果になる。コメントまで禁じる指示がある環境では上位の指示に従い、`unknown` の理由を投稿に残すことで「確認できなかった」場合と区別できる。

## 決定

- model の識別子の記載を禁止するのは **PR title** だけとする(A)。tool 名、tool 由来の prefix と並べて `doc/guidelines/pull-request-guidelines.md` の「Tool 名と model の識別子の扱い」に置く。
- PR description、PR と review のコメント、commit message(trailer を含む)、code と文書の本文では制限しない。書くかどうかは書き手の裁量とし、harness の指示に従って省いてもよい。この範囲は tool 名にも同じく適用する。
- review の canonical metadata の `Model` は必須のキーとし、実行環境で確認した識別子を書く(F)。
  - 実行環境の指示が識別子の記載を禁じていても、PR と review のコメントがその対象外と確認できる場合は、それを理由に `unknown` にしない。
  - 指示がコメントへの記載まで禁じている場合、または対象外と確認できない場合は、上位の指示を repository のルールで上書きせず `unknown` と書き、上位の指示で記載を控えた旨を同じ投稿の本文に 1 行残す。
  - それ以外で `unknown` を使うのは、識別子を確認できない場合に限る。
- 正本は、記載範囲を `doc/guidelines/pull-request-guidelines.md`、`Model` のキー定義を `.agents/skills/review-pull-request/SKILL.md` とし、両者を相互に参照する。
- 確認の手段(cloud session での session 情報の取得など)は 0061 で決める。

## 理由

禁止は、それが無いと困る場所(title の一覧性)にだけ置く。0057 と同じ考え方を model の識別子へ広げたもので、ルールを増やさずに済む。review metadata の `Model` は記録として揃うことに意味があるため、必須のキーだけは repository のルールで値の書き方を固定する。上位の指示と衝突する場合の扱いを決めておけば、agent が両方を守れない状況を作らない。

## 影響

- `doc/guidelines/pull-request-guidelines.md` の「基本方針」と「Tool 名の扱い」を改訂し、節名を「Tool 名と model の識別子の扱い」にした。
- `.agents/skills/review-pull-request/SKILL.md` の「可視 metadata の canonical フォーマット」の `Model` の項に、コメントが実行環境の指示の対象外なら `unknown` にしないこと、対象なら `unknown` とし理由を残すことを追記した。
- 正本の要約を持つ skill(`.agents/skills/release/SKILL.md` と `.agents/skills/number-working-branch-note/SKILL.md` の「参照する正本」)を同期した。0057 と同じ扱いである。
- 入口 shim(`.claude/rules/pull-request-guidelines.md`、`.cursor/rules/pull-request-guidelines.mdc`)は正本へのポインタだけを持つため、変更しない。`.github/copilot-instructions.md` は PR title と review metadata の書き方を扱っていないため、同期しない。
- 「PR title 以外では制限しない」は節全体に掛かるため、tool 名についても commit message と code / 文書の本文で制限しないことを明文化した。0057 は commit message を無規定とし、code / 文書に触れていなかった。どちらも実質の扱いは変わらない(無規定は制限しないことと同じで、文書は `.cursor/rules/` のような tool 名を含む path を元から書いている)。0057 の決定(禁止は PR title のみ)は変わらない。

## 後から見直す条件

- code や文書の本文に識別子が書かれ、陳腐化や混乱が実際に問題になったとき。
- harness の指示が PR と review のコメントにも及ぶ環境が増え、理由付きの `unknown` が増えて `Model` が記録として使えなくなったとき。
