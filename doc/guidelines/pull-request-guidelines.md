# Pull Request 作成ガイドライン

この文書は、AI agent が slapex リポジトリで Pull Request を作成または更新するときの共通正本である。

## 基本方針

- PR description は日本語で書く。
- PR title には、作成に使った tool 名やそれを示す prefix、model の識別子を含めない。
- description はレビュアーが変更意図と確認観点を把握できる粒度で書く。
- 変更内容、背景、レビュアーに特に見てほしい点、検証内容、未検証事項を分けて書く。
- 既存ルール、設定、配置、責務分担を整理した場合は、何をどの正本へ移し、どの入口をどう変更したかを具体的に書く。
- テストを実行していない場合は、理由を明記する。

## Tool 名と model の識別子の扱い

- PR title には `codex`、`claude`、`cursor` などの tool 名、`[codex]` のような tool 由来の prefix、処理に使った model の識別子を含めない。title はレビュアーが変更内容を把握するためのものである。
- PR title 以外(PR description、PR と review のコメント、commit message、code と文書の本文)では制限しない。`Co-Authored-By` や `🤖 Generated with ...` のような trailer も記載してよい。書くかどうかは書き手の裁量とする。
- 例外として、review の canonical metadata の `Model` は必須のキーであり、実行環境で確認した model の識別子を書く。agent の実行環境の指示(harness の system prompt など)が識別子の記載を禁じていても、PR と review のコメントがその禁止の対象外と確認できる場合(repository に push する成果物だけを対象とする指示など)は、それを理由に `unknown` にしない。指示がコメントへの記載まで禁じている場合、または対象外と確認できない場合は、repository のルールで上位の指示を上書きせず、`unknown` と書き、上位の指示で記載を控えた旨を同じ投稿の本文に 1 行残す。それ以外で `unknown` を使うのは、識別子を確認できない場合に限る。キーの定義と確認の手段は `.agents/skills/review-pull-request/SKILL.md` の「可視 metadata の canonical フォーマット」と「`Model` の確認手段」に従う。
- 経緯は `doc/design/decision-log/0057-pr-tool-name-restriction-scope.md` と `doc/design/decision-log/0060-model-identifier-scope.md`。

## 推奨構成

```md
## 概要

## 主な変更

## 既存整理の詳細

## レビューしてほしい点

## 検証

## 補足
```

変更が小さい場合は項目を減らしてよい。ただし、日本語で具体的に書く方針は維持する。
