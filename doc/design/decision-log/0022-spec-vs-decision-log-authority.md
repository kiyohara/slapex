# 0022 Spec vs Decision Log Authority

- 状態: decided
- 作成日: 2026-06-03
- 最終更新日: 2026-06-03
- 関連: `../../guidelines/decision-log-guidelines.md`, `0021-spec-document-split.md`

## 背景

0021 の文書分割作業中、AI agent が decision log を「仕様の正本」と記述し、「spec=正本 / decision log=参考」という建て付けを取り違えた。

原因は、この不変条件がエージェントの読む面(正本本文・`index.md`・`_template.md`・auto-load される rule shim)のどこにも明文化されていなかったことにある。正本本文は「decision log とは経緯の記録」とは述べるが、spec との権威関係(log は仕様の正本ではない)を述べていなかった。auto-load される decision-log shim も文脈に載っていたが、当該の一行が無かったため取り違えを止められなかった。

## 候補

- A. 何もしない(都度個別に修正)。
- B. `decision-log-guidelines.md`(正本)にだけ明記する。
- C. 正本に明記し、かつエージェントが着手する地点(`_template.md` / `index.md` / auto-load rule shim)にも一行ずつ置く層構造。
- D. C に加え、Copilot review の検知観点や grep / CI による機械チェックを足す。

## 検討内容

A は再発を許す。

B は正本に理屈を置けるが、エージェントは正本本文を毎回精読せず、auto-load される shim と copy する `_template.md` に依存して動く。今回も shim は文脈に載っていたが当該文言が無く、効かなかった。正本のみでは shim 依存の場面を防ぎきれない。

C は「正本に理屈」+「着手地点に一行」で予防と即時想起を両立する。MCP-first を guideline と shim の両方に置いた前例(PR #4)と整合し、`agent-configuration-management.md` の「入口は薄い shim」方針とも、単一の高シグナル一行に限る限り両立する。

D の機械チェックは自然文ゆえ脆く(正当な「正本は spec であって本ログではない」まで誤検知しうる)、Docker 前提の開発方針に対して重い。Copilot 検知は有用だが、まず予防層を固め、再発時に検討する。

## 決定

候補 C(層構造)を採用する。

- `decision-log-guidelines.md` に「正本と参照の関係」節を追加し、(1) 仕様の正本は `doc/design/` 直下の spec 文書、(2) decision log と `index.md` は参考ログで仕様の正本ではない、(3) 参照の向き、(4) 再編時は `関連` を移動先 spec へ更新し本文は履歴として据え置く、を明記する。
- `_template.md` 冒頭に、記入前に上記節を確認する旨と `関連` の向きの注記コメントを置く。
- `decision-log/index.md` 冒頭に、index と log が参考ログである旨を一行追加する。
- `.claude/rules/decision-log-guidelines.md` と `.cursor/rules/decision-log-guidelines.mdc`(auto-load される入口)に、同趣旨の一行を各 1 つ追加する。

機械チェック(grep / CI)と Copilot review 検知は今回採用せず、再発時に再検討する。

### tool 別の到達経路

auto-load shim による JIT 念押しは、機構上 Claude Code(`.claude/rules`)と Cursor(`.cursor/rules`)に限られる。Codex は AGENTS.md を入口とし、AGENTS.md の decision-log 記録ルールから `decision-log-guidelines.md` の本節へ到達する。GitHub Copilot Review も同様にリンク先正本を辿らない。

Codex / Copilot 向けに AGENTS.md へ強調一行を足すことは見送る。`42ebdb6`(MCP-first の AGENTS.md 強調を撤回し guideline + shim へ集約)で確立した「AGENTS.md を薄い index に保つ」方針と一貫させ、Codex は他の全ルールと同様 AGENTS.md→正本で到達する扱いとする。結果として、always-loaded な JIT 念押しは Claude / Cursor だけが持ち、Codex / Copilot は持たないという非対称を許容する(正本・`_template.md`・`index.md` は tool 非依存なので、ルールの実体自体は全 tool に届く)。

## 理由

取り違えは「着手地点に止める文言が無かった」ことが直接原因なので、auto-load shim と `_template.md` という*必ず目に入る面*に一行を置くのが最も効く。正本には理屈と運用ルールを集約し、shim / template / index には高シグナルの一行だけを置くことで、`agent-configuration-management.md` の「入口を薄く保つ」方針と両立させる。

「正本/入口」という一般原則は `agent-configuration-management.md` に既にあるが、それは config 資材(guidelines と shim)の軸である。spec と decision log はどちらも `doc/design/` 配下の設計文書であり軸が異なるため、`decision-log-guidelines.md` を専用の正本とする。

## 影響

- `doc/guidelines/decision-log-guidelines.md` に「正本と参照の関係」節を追加。
- `doc/design/decision-log/_template.md` 冒頭に説明コメントを追加。
- `doc/design/decision-log/index.md` 冒頭に一行追加し、本ログを「現在有効な主要方針」に登録。
- `.claude/rules/decision-log-guidelines.md` と `.cursor/rules/decision-log-guidelines.mdc` に一行追加(rule basename は不変のため AGENTS.md 変更は不要)。

## 後から見直す条件

- 上記を入れても同種の取り違えが再発する(その場合 Copilot 検知や機械チェックを検討)。
- Codex / Copilot で同種の取り違えが実際に起きる(その場合 AGENTS.md への強調一行追加を再検討する)。
- rule shim が肥大化し、一行追加が遵守率を下げる兆候が出る。
- spec / decision log の配置規約自体が変わる。
