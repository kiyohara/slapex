# 作業ブランチメモ

- ブランチ: issue-126-user-doc-style
- PR: -
- 最終更新: 2026-07-05

## 目的

Issue #126 に従い、利用者向け文書の文末・トーンを `doc/guidelines/document-style-guidelines.md` に合わせてですます調へ統一する。

## 現在の状況

- Issue #126 の依存である #125 と PR #124 は完了済み。
- `doc/help/slack-app-setup.md` の常体混在をですます調へ修正した。
- `quickstart.md` / `token-injection.md` / `README.md` は確認のみで、追加修正は不要と判断した。
- `progress.md` の #126 行を done に更新した(PR 番号は採番後に追記する)。

## 決定事項

- 手順内容・リンク構造は変更せず、文末・トーンの修正に限定する。
- `doc/help/README.md` は対象外とする。

## 次にやること

- 変更を commit / push して PR を作成する。
- PR 採番後に note を番号付きファイルへ rename し、`progress.md` の PR 欄を更新する。

## 検証

- 対象ファイルの常体文末検索: `rg -n "(である|する。|する\\)|なる。|なる\\)|できる。|使う。|渡す。|推奨する。|確認する。|参照する。|追加する。|取得する。|始まる。|残らない。|使われる。|書かない。|対象になる。|必要がある。|表示される。|作成される。|行える。|待つ。|進める。|選ぶ。|開く。|クリックする。|貼り付ける。|設定する。|反映する。)" README.md doc/help/quickstart.md doc/help/slack-app-setup.md doc/help/token-injection.md` で、本文の常体残りなし(見出し URL と「必要があります」の false positive のみ)。
- anchor 付きリンク: 対象ファイルのリンク一覧と見出し一覧を突き合わせて、既存 anchor が引き続き存在することを確認した。
- ローカルリンク先ファイル / screenshot asset: `ls` で存在確認済み。
- `git diff --check`: 成功。

## リスク・ブロッカー

- 現時点ではなし。

## セッションログ

- 2026-07-05: Issue #126 / #125 / PR #124 と関連 guideline を確認。依存完了を確認して作業ブランチを作成した。
- 2026-07-05: `doc/help/slack-app-setup.md` の文末を利用者向けのですます調に統一し、対象文書のリンク・文末確認を実施した。
