---
name: release
description: slapex の新バージョンを公開するためのリリース作業を一連の手順で安全に実施する。バージョン未指定なら直前 tag からの差分を提示して推奨版を確認し、README の version 参照・`progress.md`・decision log を 1 本のリリース PR に集約する。ユーザー merge 後に main HEAD へ署名付き tag を作成・push して goreleaser を起動し、GitHub Release / assets / checksum / `--version` / Homebrew を検証する。merge と tag push の最終実行はユーザーが行う。
---

# release

slapex の新バージョンを公開するための skill。

`v[0-9]*` tag の push をトリガに `.github/workflows/release.yml` が GoReleaser を実行し、GitHub Releases へ 4 target(darwin / linux × amd64 / arm64)の単一バイナリと `slapex_checksums.txt` を添付し、`kiyohara/homebrew-tap` の cask まで自動更新する。tag を打てば配布と Homebrew 反映は自動で走るため、この skill の責務はその前後にある。

1. リリース版の決定(指定が無ければ差分から推奨を提示)。
2. tag 前に必要な doc 調整(README の version 参照・`progress.md`・decision log)を 1 本のリリース PR に集約。
3. merge 後に署名付き tag を作成・push して GoReleaser を起動。
4. 公開物(Release / assets / checksum / `--version` / Homebrew)の検証。

PR の merge と tag の push の **最終実行はユーザーが行う**。この skill は安全側に倒し、判断に迷うときは進めずに確認する。

## 適用範囲

このリポジトリで新しいリリースを公開するときに使う。次のいずれかに該当する場合は停止し、ユーザーに状況を報告する。

- 現在の `main` が `origin/main` と乖離している、または作業ツリーに無関係な変更が残っている。
- リリース対象 commit(通常 `main` HEAD)の CI が success でない。
- 指定された、または決定したバージョンが SemVer(`vX.Y.Z`)として妥当でない、もしくは既存 tag と重複する。
- `.goreleaser.yaml` / `.github/workflows/release.yml` の構成が想定(asset 名・trigger・homebrew cask)と乖離しており、手順の前提が崩れている。

hotfix・pre-release・複数 target の同時公開など通常フローに乗らないリリースは、自動判断せずユーザーに方針を確認する。

## 参照する正本

実行前および疑問が出た時点で、以下の正本を必ず参照する。

- `doc/guidelines/git-operation-guidelines.md` — 署名付き tag 作成(`git tag -s`)・SSH remote への push の 1Password SSH agent 連携と実行環境制約。
- `doc/guidelines/github-mcp-guidelines.md` — GitHub 操作で `github-op-integrated` MCP tool を優先する方針。
- `doc/guidelines/github-cli-guidelines.md` — MCP fallback として `gh` を実行する場合の `op plugin run -- gh ...` 形式と実行環境制約。
- `doc/guidelines/pull-request-guidelines.md` — リリース PR の title / description 書式(日本語、tool 名なし)。
- `doc/guidelines/working-branch-notes-handling.md` / `doc/guidelines/working-branch-notes-security.md` — note のファイル名規約と push 前の情報統制チェック。
- `doc/guidelines/development-command-guidelines.md` — build / checksum 照合など開発コマンドを Docker Compose 経由で実行する方針。
- `doc/guidelines/decision-log-guidelines.md` — リリースに伴う方針更新を decision log に記録するときの手順。
- `.goreleaser.yaml` / `.github/workflows/release.yml` — 配布の実体(asset 名・trigger・homebrew cask 自動更新)。前提確認のため毎回読む。

## GitHub 操作形式

GitHub 操作のポリシー(MCP 優先)は常時ロードの rule にあるため再掲しない。この skill で迷いやすい運用差分だけ示す。

- PR / issue の read / write は、必ず最初に `github-op-integrated` MCP tool を試す。必要な MCP tool が現在の tools に見えていない場合は、`gh` へ進む前に利用中 agent の tool discovery 機構(利用可能なら `tool_search`)で `github-op-integrated` を検索する。見つからない場合のみ `gh` へ fallback する。
- **Release / workflow run / asset の確認系は `github-op-integrated` に該当 tool が無い**ため、`doc/guidelines/github-cli-guidelines.md` に従い `gh`(`.op/` と `op` が使える場合は `op plugin run -- gh ...`)で行う。`gh release view` / `gh run watch` / `gh run list` などが該当する。
- write 系を `gh` に fallback する場合は、`doc/guidelines/github-mcp-guidelines.md` の write fallback 注意に従い、再実行前に read 系で対象の現状を確認してから実行する。
- `git commit` / `git tag -s` / `git push`(SSH remote)は MCP に寄せず `doc/guidelines/git-operation-guidelines.md` に従う。署名失敗・1Password 承認プロンプト不達・socket 通信エラーが起きた場合は、制約のない実行環境で同じコマンドを再実行する。

## 手順

以下の順で実行する。各ステップで失敗・矛盾を検出したら停止し、ユーザーに報告する。

### 1. 前提チェック

```sh
git branch --show-current
git fetch origin
git status
```

- 現在ブランチが `main` で、`origin/main` と同一 commit であることを確認する。乖離・未コミット変更があれば停止。
- リリース対象 commit(通常 `main` HEAD)の CI が success であることを確認する(`gh run list --branch main --limit 5` 等。Release / workflow 系は `gh`)。
- success でない、または対象 commit が判別できない場合は停止し、ユーザーに確認する。

### 2. バージョン決定

- バージョン指定がある場合は、それを `vX.Y.Z` 形式に正規化する。
- 指定が無い場合は、直前のリリース tag からの差分を提示してユーザーに確認する。

```sh
git describe --tags --abbrev=0   # 直前 tag
git log "$(git describe --tags --abbrev=0)"..HEAD --oneline
```

  - 直前 tag から HEAD までの commit / マージ済み PR の一覧を提示する。
  - 既定は **patch bump** を推奨する。breaking change や新機能の兆候があれば minor / major を提案し、根拠を添えて提示する。最終判断はユーザーに委ねる(本リポジトリは Conventional Commits を使わないため、機械的に確定しない)。
- 決定したバージョンが SemVer かつ `v` 接頭辞付きで、既存 tag(`git tag --list`)と重複しないことを確認する。

### 3. リリース用ブランチと作業 note

- `main` からリリース用ブランチを作成する(例: `release/vX.Y.Z`)。
- `doc/guidelines/working-branch-notes-handling.md` に従い `working-branch-notes/draft_<escaped-branch>.md` を作成する。目的・対象バージョン・参照した差分の要点を記録する。

### 4. doc 更新(1 PR に集約)

tag を打つ前に必要な doc 調整を、このリリース用ブランチにまとめる。

- **README**: install 例の version 参照を新版へ bump する。直書き箇所が増減している可能性があるため、置換前に現行版を洗い出す。

```sh
grep -n "$(git describe --tags --abbrev=0)" README.md
```

  - 旧版を直書きしている install 例(`--version <旧版>` の一行、`VERSION=<旧版>` の例など)を新版に置き換える。汎用 placeholder(`<version>` 等)は触らない。
- **`progress.md`**: リリース行を追加 / 更新する。
- **decision log**: 配布方式・導入手段など既存 log に、新版での確認事項を追記する必要があるかを `doc/guidelines/decision-log-guidelines.md` に従って判断する。新規 log の要否を含め、判断に迷う場合はユーザーに確認する。
- **コードのバージョン文字列は変更しない**。version は `-X main.version` で tag から注入されるため(`.goreleaser.yaml`)、ソース修正は不要。

### 5. PR 作成

- 情報統制チェック(`doc/guidelines/working-branch-notes-security.md`)を通したうえで、`doc/guidelines/pull-request-guidelines.md` に従い PR を作成する(MCP 優先)。
- `number-working-branch-note` skill で draft note を採番する。
- `git commit` / `git push` は `doc/guidelines/git-operation-guidelines.md` に従い、push はユーザー確認後に行う。

### 6. merge 待ち(ユーザー)

- PR の merge は **ユーザーが行う**。ここで一旦停止し、merge を待つ。merge 前に tag を打たない。

### 7. tag 作成・push(merge 後 / ユーザー承認)

```sh
git switch main
git pull origin main
git log -1 --oneline   # bump を含む merge commit が HEAD か確認
```

- `main` HEAD が README bump を含む merge commit であることを確認する。
- 署名付き tag を作成する。

```sh
git tag -s vX.Y.Z -m "vX.Y.Z"
```

- push は SSH remote 経由で 1Password SSH agent を使うため、**実行コマンドを提示してユーザー承認を取ってから**行う。

```sh
git push origin vX.Y.Z
```

- 署名 / SSH の阻害が起きた場合は `doc/guidelines/git-operation-guidelines.md` に従い、制約のない実行環境で再実行する。

### 8. GoReleaser workflow の監視

- tag push をトリガに走る Release workflow の success を確認する(`gh run watch <run-id> --exit-status` 等。`gh` 系)。
- 失敗時は log を確認し、ユーザーに報告する。

### 9. 検証

Release / asset 確認は MCP 非対応のため `gh` で行う。build / checksum 照合は Docker Compose 経由(`doc/guidelines/development-command-guidelines.md`)。

- `gh release view vX.Y.Z` で draft / prerelease ではなく公開済みであり、5 assets(`slapex_darwin_amd64` / `slapex_darwin_arm64` / `slapex_linux_amd64` / `slapex_linux_arm64` / `slapex_checksums.txt`)が添付されていることを確認する。
- 任意の 1 つの linux asset と `slapex_checksums.txt` を download し、dev コンテナで checksum を照合する。

```sh
docker compose run --rm dev sh -c 'sha256sum --ignore-missing -c slapex_checksums.txt'
```

- 同じ linux バイナリを dev コンテナで実行し、`--version` が新版を返すことを確認する。
- `kiyohara/homebrew-tap` の `Casks/slapex.rb` が新版へ自動更新されたことを確認する。
- **ユーザー分担**(agent 環境では実施できない): macOS バイナリの `--version`、`brew update && brew upgrade --cask slapex && slapex --version`。これらはユーザーに依頼する。

### 10. 検証結果の記録(別 note / PR)

- 準備 PR は tag を打つ前に merge 済みのため、検証結果の記録は **別ブランチ / 別 note / 別 PR** で行う(過去の運用と同様)。
- `progress.md` と decision log の該当箇所を、新版での確認結果として完了状態に更新する。

## 終了時の報告

ユーザーへの最終報告には次を含める。

- 決定したバージョンと、その根拠(直前 tag からの差分の要点)。
- 作成した PR / tag / commit(分かれば SHA)。
- 検証結果(Release assets / checksum / linux `--version` / Homebrew cask)。
- ユーザー分担として残っている確認項目(macOS `--version`、`brew upgrade`)。
- merge / tag push の状況(ユーザー承認の結果)。

## やらないこと

- PR の merge と tag push の最終実行(いずれもユーザーが行う)。
- リリース告知・SNS への投稿。
- `kiyohara/homebrew-tap` repo の手動編集(GoReleaser の cask 自動更新に委ねる)。
- 署名・公証(notarization)対応。
- CHANGELOG の新設(release notes は GoReleaser の自動生成に委ねる)。
- コードのバージョン文字列の直接書き換え(`-X main.version` で注入される)。
- 機械的なバージョン番号の自動確定(差分を提示し、最終判断はユーザーに委ねる)。
