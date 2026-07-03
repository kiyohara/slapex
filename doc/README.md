# doc

このディレクトリには、リポジトリ全体で共有するドキュメントを置く。

## 配置

| path | 置くもの |
|---|---|
| `doc/guidelines/` | AI agent と人間が共通で従う作業ルール、運用ガイドライン |
| `doc/design/` | `slapex` の仕様設計、利用体験設計、設計判断の記録 |
| `doc/help/` | 利用者が GitHub 上で直接読む help / how-to |
| `doc/samples/` | 生成済みサンプル export(架空データ。`tools/gensample` で再生成) |

作業状況の一覧は `progress.md`、ブランチ単位の作業メモは `working-branch-notes/` に置く。

開発作業を始めるときは、GitHub Issue / `progress.md` / skill / PR / release の流れをまとめた `doc/guidelines/development-loop.md` を入口として読む。

## 方針

AI agent 専用の説明を別ファイルとして分離せず、人間も読める `README.md` と `doc/guidelines/` を共通正本として扱う。

各ディレクトリに新しい文書を追加する前に、そのディレクトリの `README.md` を確認する。
