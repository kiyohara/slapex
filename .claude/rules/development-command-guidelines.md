# 開発コマンド実行ルール

- 詳細は `doc/guidelines/development-command-guidelines.md`
- 依存 install / アプリ起動 / test / build など、開発環境を host OS 上に構築・実行する類のコマンドは Docker Compose 経由を原則とする
- host OS 側で直接実行する前に、Docker 起動可否を確認し、必要ならユーザー承認を得る
- 実装スタックは未確定のため、特定スタック固有のコマンド名を前提化しない
- tool 固有入口だけに恒久ルールを書かない
