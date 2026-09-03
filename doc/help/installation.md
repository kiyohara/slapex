# インストール

このページは、`slapex` のインストール手順の詳細をまとめたものです。macOS / Linux(それぞれ amd64 / arm64)に対応しています(Windows は初期対象外)。

macOS では Homebrew Cask、macOS / Linux 共通では install script を使えます。1 ステップずつ確認したい場合は手動手順を使ってください。

## Homebrew Cask(macOS)

Homebrew を使う場合は、tap から cask としてインストールします:

```sh
brew install --cask kiyohara/tap/slapex
```

インストール後の確認:

```sh
slapex --version
```

## クイックインストール(install script)

最新リリースを取得し、sha256 checksum を検証して `/usr/local/bin` に配置します:

```sh
curl -fsSL https://raw.githubusercontent.com/kiyohara/slapex/main/scripts/install.sh | sh
```

バージョンやインストール先を指定する場合は、パイプに `-s --` でオプションを渡します:

```sh
curl -fsSL https://raw.githubusercontent.com/kiyohara/slapex/main/scripts/install.sh \
  | sh -s -- --version v1.2.1 --bin-dir "$HOME/.local/bin"
```

スクリプトは OS / arch を自動判定し、`slapex_<os>_<arch>` と `slapex_checksums.txt` を取得して checksum を照合してから配置します。`/usr/local/bin` に書き込めない場合は sudo を使うか、`--bin-dir` で書き込み可能なディレクトリを指定してください。全オプションは `--help`、実際の取得先を確認するだけなら `--dry-run` で表示できます。

> 実行前にスクリプト内容を確認したい場合は、上記 URL を開くか `curl -fsSLO <URL>` で取得してから `sh install.sh` を実行してください。

## 手動インストール(詳細版)

[GitHub Releases](https://github.com/kiyohara/slapex/releases) から、OS / arch に合うバイナリをダウンロードします。配布物は単一バイナリと sha256 checksum(`slapex_checksums.txt`)です。

| OS | arch | asset 名 |
|---|---|---|
| macOS (Apple Silicon) | arm64 | `slapex_darwin_arm64` |
| macOS (Intel) | amd64 | `slapex_darwin_amd64` |
| Linux | x86_64 | `slapex_linux_amd64` |
| Linux | arm64 | `slapex_linux_arm64` |

まずバイナリと checksum を取得します(`<version>` は対象のリリース tag、`ASSET` は上の表から自分の OS / arch に置き換える):

```sh
VERSION=<version>          # 例: v1.2.1
ASSET=slapex_darwin_arm64  # 上の表から自分の OS / arch に合わせて選ぶ
BASE="https://github.com/kiyohara/slapex/releases/download/${VERSION}"

curl -LO "${BASE}/${ASSET}"
curl -LO "${BASE}/slapex_checksums.txt"
```

次に checksum を確認します。コマンドは OS で異なります(対象 asset の行だけ検証):

```sh
# macOS
shasum -a 256 -c <(grep " ${ASSET}\$" slapex_checksums.txt)
```

```sh
# Linux
sha256sum -c <(grep " ${ASSET}\$" slapex_checksums.txt)
```

最後に実行権限を付与し、PATH 上に `slapex` として配置します:

```sh
chmod +x "${ASSET}"
mv "${ASSET}" /usr/local/bin/slapex
```

インストール後の確認:

```sh
slapex --version
```

## 次のステップ

- 初めて使う場合はチェックリスト形式の [クイックスタート](quickstart.md) に沿って進めてください。
- インストール直後に token なしで動作を試すには、[使い方](usage.md#token-なしで試す--demo) の `--demo` を使ってください。
