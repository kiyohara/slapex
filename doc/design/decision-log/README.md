# decision-log

このディレクトリには、設計判断や方針変更の経緯を 1 テーマ 1 ファイルで記録する。

記録ルールの正本は `doc/guidelines/decision-log-guidelines.md`。

## 入口

- `index.md`: 現在有効な主要方針と未決事項の入口
- `_template.md`: 新しい decision log を作るときのひな形
- `<連番>-<短い英語slug>.md`: 個別の詳細ログ

## 注意

- 新しい decision log を作る前に、まず `index.md` を読む。
- 詳細な議論は個別ログに置き、`index.md` には現在有効な結論と参照だけを置く。
- Slack token、個人情報、顧客固有情報、出力 HTML に含まれる機密情報を書かない。
