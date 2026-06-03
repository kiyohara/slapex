# 0019 document directory structure

- 状態: decided
- 作成日: 2026-06-03
- 最終更新日: 2026-06-03
- 関連: `doc/README.md`, `doc/design/README.md`, `doc/help/README.md`, `progress.md`

## 背景

従来は、仕様設計、decision log、progress、利用者向け help が `doc/product/` 配下にまとまっていた。

しかし、`doc/product/` という名前は用途が曖昧であり、設計検討文書、作業状況、利用者向け手順が同じ階層に混ざって見える問題があった。

## 候補

- `doc/product/` を維持する。
- `doc/development/` 配下に decision log、specification、progress を置き、`doc/help/` を分ける。
- `doc/design/` 配下に仕様設計と decision log を置き、利用者向け help は `doc/help/`、作業状況は root の `progress.md` に置く。
- `doc/spec-design/` のように仕様と設計の両方を名前に含める。

## 検討内容

`product` は「プロダクトに関係する文書」という意味では広いが、利用者向け help と内部の設計検討が混ざりやすい。

`development` は実装や開発作業の文脈が強く、decision log や仕様設計も含められるが、利用者体験や出力仕様の設計文書を置く名前としてはやや広い。

`design` は、仕様を決めるための検討文書と decision log を置く場所として自然である。decision log も design decision の履歴と考えれば、`doc/design/decision-log/` に置いて違和感が少ない。

`progress.md` は設計文書ではなく作業状況の管理表であるため、`doc/design/` 配下ではなく root に置く方が役割が明確になる。

`spec-design` は意図は伝わるが、一般的な名前ではなく、最終仕様なのか設計検討なのかがかえって曖昧になる。

## 決定

ドキュメント構成は次のようにする。

```text
doc/
├── guidelines/
├── design/
│   ├── decision-log/
│   └── usage-flow.md
└── help/

progress.md
working-branch-notes/
```

`doc/guidelines/` には、AI agent と人間が共通で従う作業ルール、運用ガイドラインを置く。

`doc/design/` には、`slapex` の仕様設計、利用体験設計、設計判断の記録を置く。

`doc/help/` には、利用者が GitHub 上で直接読む help / how-to を置く。

`progress.md` は root に置き、横断的な作業状況の管理表として扱う。

`working-branch-notes/` は、ブランチ単位の作業目的、状況、判断、引き継ぎメモを扱う。

## 理由

設計文書、利用者向け help、作業状況を分離することで、文書を追加するときの判断がしやすくなる。

`doc/design/` は decision log と仕様設計を一体で扱えるため、設計判断の履歴と現在の仕様素案を辿りやすい。

`doc/help/` を分けることで、CLI から案内する GitHub URL が利用者向け文書であることを明確にできる。

## 影響

`doc/product/usage-flow.md` は `doc/design/usage-flow.md` に移動する。

`doc/product/decision-log/` は `doc/design/decision-log/` に移動する。

`doc/help/slack-app-setup.md` を新規作成する（旧構成には help 用ファイルが無いため、移動ではなく新設）。

`doc/product/progress.md` は root の `progress.md` に移動する。

`AGENTS.md`、`doc/guidelines/decision-log-guidelines.md`、tool 固有 rule、working branch notes、GitHub Copilot instructions の参照パスを更新する。

各ディレクトリには、人間と AI agent の両方が読める `README.md` を置き、配置判断の入口とする。AI agent 専用の別ガイドラインを増やさず、必要な恒久ルールだけ `doc/guidelines/` に置く。

## 後から見直す条件

設計文書が増え、`doc/design/` 直下が見通しにくくなった場合は、`doc/design/cli.md`、`doc/design/output-format.md` などへ分割する。

利用者向け help が増えた場合は、`doc/help/how-to-use.md` を入口として追加する。
