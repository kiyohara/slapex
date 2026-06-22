# 0034 配布方式

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-22
- 関連: `doc/design/architecture.md`, `doc/design/decision-log/0031-supported-platforms.md`, `doc/design/decision-log/0032-implementation-language.md`

## 背景

配布形態として単一バイナリを重視する方針をユーザーに確認した。具体的な配布チャネルと build target を決め、言語選定(0032)の前提と整合させる必要がある。

## 候補

- GitHub Releases に multi-platform バイナリを添付する(リリース自動化は goreleaser)。
- Homebrew tap を最初から提供する。
- パッケージレジストリ(npm / PyPI 等)経由の配布。
- コンテナ image としての配布。

## 検討内容

- GitHub Releases + 単一バイナリは、ローカル導入(download して PATH に置く)と CI 導入(release URL から取得)の両方を最小手順で満たす。goreleaser はクロスコンパイル、archive、checksum、Releases 添付までを一括自動化でき、Go との組み合わせで実績が厚い。
- Homebrew tap は macOS 利用者の導入体験を上げるが、tap リポジトリの維持が必要。Releases が先にあれば後から追加できるため、初期必須ではない。
- パッケージレジストリ配布はランタイム前提になりやすく、単一バイナリ方針と合わない。
- コンテナ image 配布は CI 用途では便利だが、ローカルの TTY interactive 利用と相性が悪く、主配布経路にはしない。

## 決定

- 配布は GitHub Releases への単一バイナリ添付を主経路とする。
- build target は `darwin/amd64` / `darwin/arm64` / `linux/amd64` / `linux/arm64`(0031 の対象プラットフォーム)。
- リリース自動化は goreleaser を使い、`goreleaser` 設定で 4 target の単一バイナリと sha256 checksum を生成する。version は build 時の ldflags で CLI に埋め込む。
- Homebrew tap は将来検討として未決事項に記録する。コンテナ image 配布は採用しない。
- リリース workflow と tap の整備は PoC 後の実装フェーズで行う。

## 理由

- 単一バイナリ方針の利点(導入の軽さ)を最短で利用者に届けられ、維持コストも小さいため。

## 影響

- `architecture.md` の配布方式に反映した。
- 実装フェーズで goreleaser 設定と GitHub Actions release workflow を追加した。
- `README.md` に GitHub Releases からの install 手順、checksum 確認、`--version` 確認を追加した。
- v1.0.0 は GitHub Releases に `darwin/amd64` / `darwin/arm64` / `linux/amd64` / `linux/arm64` の単一バイナリと `slapex_checksums.txt` を添付して公開した。
- 導入補助手段(install script の先行採用、Homebrew tap の後続実装)の方針は [0041-install-convenience.md](0041-install-convenience.md) に分離して記録した。本ログの主経路(GitHub Releases 単一バイナリ)は維持する。

## 後から見直す条件

- Homebrew tap の需要が確認できた場合(未決事項から個別ログへ)。
- 署名・公証(macOS notarization)が配布上必要になった場合。
- Windows 対応(0031)を行う場合の target 追加。
