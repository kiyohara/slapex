# 開発コマンド実行ルール

- 詳細は `doc/guidelines/development-command-guidelines.md`
- Rails / Bundler / Node.js / Yarn 系コマンドは Docker Compose 経由を原則とする
- host OS 側で `bundle` / `npm` / `yarn` を直接実行する前に、Docker 起動可否を確認し、必要ならユーザー承認を得る
- tool 固有入口だけに恒久ルールを書かない
