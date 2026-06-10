# 作業ブランチメモ

- ブランチ: finalize-detailed-specs
- PR: (未採番)
- 最終更新: 2026-06-10

## 目的

詳細仕様の確定(仕様確定 → アーキテクチャ選定 → PoC 実装、の 3 ステップ作業の Step 1)。後続のアーキテクチャ選定と PoC 実装に必要な範囲に絞って仕様を確定し、将来検討に回す項目は未決事項として明示的に記録する。

## 現在の状況

- 仕様文書 2 件を新設(`doc/design/cli-interface.md` / `doc/design/slack-api-usage.md`)。
- 既存 4 文書(`usage-flow.md` / `output-format.md` / `html-rendering.md` / `cache.md`)を確定内容で更新。
- decision log 0024〜0031 を追加し、`index.md` の主要方針・未決事項を更新。

## 決定事項

- 作業全体の前提(ユーザー確認済み):
  - 配布は単一バイナリ(GitHub Releases / Homebrew 等)を重視する。
  - 言語・フレームワーク選定はユーザーの慣れを基準に入れず完全委任。
  - PoC は実 workspace の bot token を用意してもらい E2E 検証する。
- CLI option / exit code / 入出力ストリーム / token 受け渡し(環境変数のみ)を確定(0024)。
- Slack API 利用方針(method 一覧、pagination、rate limit 対応、user / emoji / file 解決)を確定(0025)。2025 年の非 Marketplace 配布アプリ向け rate limit 強化は internal App が対象外であることを公式 changelog で確認し、decision 0009 の裏づけとして記録。
- mrkdwn → HTML 変換とサニタイズ方針(0026)、message subtype 表示方針(0027)、時刻表示とタイムゾーン(0028)、directory label 正規化規則(0029)、cache schema と再利用検証(0030)、対象プラットフォーム(0031)を確定。
- 未決事項として将来検討に残したもの: rich_text blocks の完全レンダリング、出力制御 option(--quiet / --verbose 等)、user 解決の大量呼び出し最適化、Windows 対応、差分取得、CI artifact 化、guard option、thread replies を含む全体取得量上限。

## 次にやること

- PR 作成、採番後に note rename、自己マージ。
- Step 2(アーキテクチャ選定)へ。

## 検証

- 仕様文書間の相互参照、decision log index の参照先・連番の整合を確認。
- 本ブランチは文書のみの変更であり、実装コード・テストは扱っていない。

## リスク・ブロッカー

- 仕様は実装前の机上確定であり、PoC(Step 3)で実 API との齟齬が見つかった場合は decision log を更新して仕様を修正する前提。

## セッションログ

- 2026-06-10: ユーザーへ 3 点確認(配布形態 / 言語選定での慣れの扱い / PoC 検証方法)し、回答(単一バイナリ / 完全委任 / 実 token E2E)を得て作業開始。`finalize-detailed-specs` ブランチを作成。
- 2026-06-10: 仕様文書の新設・更新と decision log 0024〜0031 を作成。
