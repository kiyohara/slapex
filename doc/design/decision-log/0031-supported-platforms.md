# 0031 対象プラットフォーム

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-10
- 関連: `doc/design/cli-interface.md`, `doc/design/output-format.md`

## 背景

TTY の interactive selection、ファイル名の正規化、配布形式の検討には、対象 OS の確定が必要だった。これまでの仕様は暗黙に Unix 系を前提としており、明文化されていなかった。

## 候補

- macOS / Linux のみを対象にする。
- Windows も最初から対象にする。

## 検討内容

- 主要な利用シナリオ(ローカル実行は macOS、CI は GitHub Actions の Linux runner)は macOS / Linux で完結する。
- Windows 対応は、ファイル名の追加制約(予約名、末尾 dot / space)、path 長制限、TTY / コンソール挙動の差異、配布 binary の追加 target など、初期スコープに対してコストが大きい。
- 単一バイナリ配布を重視する方針のため、将来 Windows target を追加すること自体は配布側の仕組みで対応しやすい。初期から仕様で縛る必要はない。

## 決定

- 対象プラットフォームは macOS と Linux とする。CI は GitHub Actions の Linux runner を想定する。
- Windows は初期対象外とし、将来検討として `index.md` の未決事項に記録する。
- directory label の正規化(0029)は Windows の禁止文字も置換対象に含めており、将来対応の障害を増やさない。

## 理由

- 現在の利用シナリオに対して十分で、初期実装と検証のコストを抑えられるため。

## 影響

- `cli-interface.md` に対象プラットフォームを明記した。
- アーキテクチャ選定では macOS / Linux 向けクロスコンパイルを必須要件、Windows target は加点要素として扱う。

## 後から見直す条件

- Windows 利用者からの需要が確認できた場合。
- 配布の仕組み(GitHub Releases など)が整い、Windows target の追加コストが binary build の追加だけになった場合。
