# 0002 開発基盤として Docker / Docker Compose を前提とする

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `doc/guidelines/development-command-guidelines.md`

## 背景

本リポジトリでは AI agent と人間が共同で開発を進める。その過程で、依存パッケージの install、アプリケーションの実行、test、asset build といった開発コマンドを実行する必要がある。

これらを host OS 上で直接実行すると、次の問題がある。

- 開発者・AI agent ごとに環境差異が生じ、再現性が下がる。
- 実装スタックが未確定な段階で host OS を汚す。
- サプライチェーン攻撃のリスクがある。悪意ある依存パッケージや install 時の script(postinstall 等)が、host OS 上の認証情報・SSH 鍵・環境変数・他プロジェクトのファイルへ到達しうる。

実運用上はすでに `doc/guidelines/development-command-guidelines.md` で Docker Compose 前提として AI agent を規律しているが、これを確定方針として裏づける decision log が無かった。PR #1 の Copilot review でも、「実装技術が未確定」という他ドキュメントの表現との整合が問われた。

## 候補

- A: 開発基盤として Docker / Docker Compose を前提とし、依存 install・実行・test・build はコンテナ内で行う。
- B: host OS 上に開発環境を直接構築し、ツールチェーンを直接利用する。
- C: 方針を固定せず、その都度判断する。

## 検討内容

- A の利点: 環境再現性が高い。host OS を汚さない。サプライチェーン攻撃の影響範囲をコンテナ境界に限定でき、host の認証情報・SSH 鍵・環境変数・他プロジェクトへの横展開を抑止しやすい。AI agent の自動実行とも相性が良く、実行境界が明確になる。欠点: Docker / Docker Compose が利用できる環境を前提とする。初期構築コストがかかる。コンテナ外への副作用(volume mount や共有 socket 等)は別途注意が必要。
- B の利点: 手軽で、ツールチェーンを直接叩けるため速い。欠点: 環境差異が出やすい。host を汚す。悪意ある install script や依存が host 全体・他プロジェクト・認証情報へ到達しうる。特に AI agent に host 上での任意の install を許すリスクが大きい。
- C の欠点: 判断がぶれ、同じ議論を繰り返す。AI agent ごとに挙動が分かれる。

実装スタック(言語・フレームワーク・パッケージ管理ツール)は未確定だが、それと「開発基盤を Docker / Docker Compose にする」ことは独立に決められる。基盤を先に固定しても、後続の実装スタック選択を縛らない。

## 決定

開発基盤として Docker / Docker Compose を前提とすることを確定方針とする。

- AI agent は、依存 install・アプリケーションの実行・test・asset build を原則 Docker Compose 経由で行い、host OS 上で直接実行しない。
- 具体的な service 名・コマンドは、実装スタックと Compose 構成が確定した時点で `doc/guidelines/development-command-guidelines.md` で具体化する。

## 理由

- サプライチェーン攻撃の影響範囲をコンテナ境界に閉じ込められ、host OS 上の認証情報・SSH 鍵・環境変数・他プロジェクトへの横展開リスクを下げられる。
- 環境再現性が上がり、AI agent と人間、複数 agent 間で挙動を揃えやすい。
- 実装スタックが未確定でも、開発基盤の方針は独立に固定でき、AI agent の実行境界を明確にできる。

## 影響

- `doc/guidelines/development-command-guidelines.md` の「Docker Compose 前提」を確定方針として裏づける。実装スタック固有のコマンド名を前提化しない方針はそのまま維持する。
- `doc/design/usage-flow.md` や `README.md` の人間向け手順を整備する際は、Docker Compose を前提に記述する。
- 実装スタック確定時に、service 構成と具体コマンドを `development-command-guidelines.md` で更新する。
- `doc/design/decision-log/index.md` の「現在有効な主要方針」に追加する。

## 後から見直す条件

- Docker / Docker Compose を利用できない実行環境が主流になった場合。
- コンテナ化が開発効率やツール連携を著しく阻害すると判明した場合。
- サプライチェーン対策として、より強い隔離手段(より厳格な sandbox など)へ移行する方針になった場合。
