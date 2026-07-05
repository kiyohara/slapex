<p align="center">
  <img src="assets/slapex-logo-shadow.svg" width="160" height="160" alt="slapex logo">
</p>

# slapex

`slapex` は、Slack channel の履歴を、外部 URL に依存せずローカルで閲覧できる静的 HTML + assets 一式として export(書き出し)する CLI です。投稿・スレッドの返信に加え、標準 / カスタム絵文字、reaction、添付ファイル、URL unfurl の preview 画像もまとめて取得します。

対象プラットフォームは macOS と Linux(それぞれ amd64 / arm64)です。

## ターミナルでの実行例

<p align="center">
  <img src="assets/demo/slapex-demo-ja.gif" width="760" alt="slapex をターミナルで実行する様子(token 入力、channel 選択、進捗表示)のデモ"><br>
  <sub>token の対話入力、channel の選択、進捗表示、完了までの流れ</sub>
</p>

完了時には出力先ディレクトリの絶対 path が表示されます。その配下に、ローカルで閲覧できるファイル群が生成されます。

<p align="center">
  <img src="assets/screenshots/output-dir-finder.png" width="760" alt="ファイルマネージャーで slapex の出力ディレクトリを開いた例"><br>
  <sub>出力先ディレクトリに生成されるファイル群</sub>
</p>

## 出力プレビュー

<p align="center">
  <kbd><img src="assets/screenshots/sample-timeline-ja.png" alt="サンプル export のタイムライン表示"></kbd><br>
  <sub>タイムライン表示(日付区切り、システムメッセージ、mrkdwn 装飾、メンション、絵文字、reaction、画像、URL unfurl)</sub>
</p>

<p align="center">
  <kbd><img src="assets/screenshots/sample-thread-ja.png" alt="サンプル export のスレッドと添付ファイル表示"></kbd><br>
  <sub>スレッド、コードブロック、bot 投稿、添付ファイル</sub>
</p>

Slack App や token を用意する前に試す場合は、`--demo` で同梱サンプルから HTML export を生成できます。

```sh
slapex --demo --output ./slapex-demo
```

リポジトリを clone して [`doc/samples/ja/index.html`](doc/samples/ja/index.html) をブラウザで開くと、この出力をそのまま閲覧できます。[^sample-data]

## クイックスタート

初めて使う場合は、チェックリスト形式の [クイックスタート](doc/help/quickstart.md) に沿って進めると、インストールから初回 export・閲覧までを 1 ページで完走できます(所要 15 分程度)。

インストールだけ先に済ませる場合(macOS の例):

```sh
brew install --cask kiyohara/tap/slapex
```

Linux や install script、手動インストール(checksum 検証)などその他の方法は [インストール](doc/help/installation.md) を参照してください。

## 利用方法の詳細

用途別の詳細は次の help を参照してください。

- **Slack のセットアップ**: [Slack App 準備手順](doc/help/slack-app-setup.md) — App の作成、scope 設定、install、token 発行。
- **Token の渡し方**: [Token の渡し方](doc/help/token-injection.md) — 実行時の貼り付け(基本)、secret manager、CI secrets。
- **インストール**: [インストール](doc/help/installation.md) — Homebrew Cask、install script、手動手順と checksum 検証。
- **使い方**: [使い方](doc/help/usage.md) — 実行方法、option 一覧、cache、出力の構造。
- **制限事項・FAQ**: [よくある質問・制限事項](doc/help/faq.md) — 取得範囲の default、再現されない表示、つまずいたときの対処。

## 開発者向けドキュメント

- ドキュメント配置の入口: [`doc/README.md`](doc/README.md)
- AI agent / 開発者向け共通入口とガイドライン: [`AGENTS.md`](AGENTS.md)

## ライセンス

MIT License。詳細は [`LICENSE`](LICENSE) を参照してください。

[^sample-data]: `--demo` とスクリーンショットは、同梱の生成済みサンプル export と同じ架空データを使います。
