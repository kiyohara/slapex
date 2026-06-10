# 0032 実装言語の選定

- 状態: decided
- 作成日: 2026-06-10
- 最終更新日: 2026-06-10
- 関連: `doc/design/architecture.md`, `doc/design/decision-log/0031-supported-platforms.md`

## 背景

詳細仕様(PR #9)の確定により、実装言語を決める材料が揃った。選定に先立ちユーザーに確認した前提は次のとおり。

- 配布形態は単一バイナリ(GitHub Releases / Homebrew 等で第三者が導入、CI への導入も容易)を最重要基準とする。
- 作者の言語の慣れは選定基準に入れない(試作 slack_posts_dumper は Python 製だが加点しない。0001 のとおり試作の方針は暗黙採用しない)。

評価軸は、(1) 単一バイナリ配布とクロスコンパイル、(2) 標準ライブラリの守備範囲と依存最小化(サプライチェーンリスク、0002 と同根)、(3) 本ツールの要件(HTTP / JSON / HTML 自動エスケープ / TTY UI / ファイル埋め込み)への適合、(4) リリース自動化の実績、(5) Docker 内開発のイテレーション速度、とした。

## 候補

- Go
- Rust
- TypeScript(Deno compile / Bun compile による単一ファイル化)
- Python(PyInstaller / Nuitka 等によるバイナリ化)
- Ruby

## 検討内容

- **Go**: `GOOS` / `GOARCH` 指定と `CGO_ENABLED=0` だけで全 target をどの環境からもクロスコンパイルできる。標準ライブラリに `net/http` / `encoding/json` / `html/template`(contextual auto-escaping)/ `flag` / `embed` が揃い、本ツールの中核要件を外部依存なしで満たせる。バイナリは 10MB 前後。goreleaser による Releases 自動化が成熟。ビルドが速く Docker 内開発のイテレーションが軽い。gofmt により整形が一意。
- **Rust**: バイナリサイズと実行性能は最良だが、本ツールは Slack API のネットワーク律速であり性能差が利点にならない。HTTP(reqwest)/ JSON(serde)/ テンプレート(askama 等)/ 非同期 runtime(tokio)がすべて外部 crate で、依存ツリーが Go 構成より大きくなる。クロスコンパイルは target toolchain の準備が必要。コンパイル時間も長く、イテレーションが重い。
- **TypeScript(Deno / Bun)**: `deno compile` / `bun build --compile` で単一ファイル化できるが、ランタイム同梱でバイナリが 60〜100MB 級になり、「軽く導入できる単一バイナリ」という配布意図に合わない。配布実績・成熟度でも Go / Rust に劣る。
- **Python**: 試作の知見はあるが、単一バイナリ化(PyInstaller 等)は生成物が大きく起動も遅い。クロスコンパイルができず、target OS ごとのビルド環境が必要になり、配布基準で大きく不利。
- **Ruby**: 単一バイナリ化の標準的な手段が乏しく、配布基準で同様に不利。

## 決定

実装言語は Go(現行安定版 1.26 系)とする。

## 理由

- 最重要基準である単一バイナリ配布とクロスコンパイルが言語標準機能で完結し、他候補に対して明確に優位。
- 標準ライブラリの守備範囲が本ツールの要件とよく一致し、外部依存を TTY UI 系に限定できる(依存方針は 0033)。
- `html/template` の contextual auto-escaping が、確定済みのサニタイズ方針(0026)を実装レベルで支える。

## 影響

- `doc/design/architecture.md` を新設し、実装アーキテクチャの正本とする。
- 各仕様文書の「実装アーキテクチャは未確定」の前置きを `architecture.md` への参照に更新する。
- 開発環境(Docker Compose の service 構成)は PoC ブランチで `golang` 公式 image を base に整備する。
- PoC(Step 3)で、この選定の機能充足性(API 取得、TTY 選択、HTML 生成、単一バイナリ build)を実 token の E2E で確認する。

## 後から見直す条件

- PoC で Go では仕様の実現が困難な領域が見つかった場合。
- 配布形態の前提(単一バイナリ重視)が変わった場合。
