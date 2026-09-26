# Git 操作ルール

この文書は、slapex リポジトリで AI agent が `git` コマンドを実行するときの共通ルールである。

## 1Password 連携が必要な操作

このリポジトリでは、次の `git` 操作で 1Password との連携が必要になる場合がある。

- 署名（commit / tag）を伴う操作:
  - `git commit`
  - merge commit を作る `git merge`
  - `git rebase --continue`
  - `git cherry-pick`
  - `git tag -s`
- GitHub の SSH remote への通信を伴う操作:
  - `git push`
  - `git fetch`
  - `git pull`
  - `git ls-remote`
  - `git clone`

  remote URL が HTTPS の場合、これらの操作は 1Password SSH agent 連携の対象外とする。

これらは 1Password との連携に依存するため、AI agent の実行環境、sandbox、権限分離、TTY 設定などにより、socket 通信や承認プロンプトが阻害される場合がある。SSH remote への通信は 1Password SSH agent を使う。署名の経路は `gpg.ssh.program` の設定で変わり、SSH agent を経由しない場合がある(「署名の実行経路」)。

各 AI agent は、現在の実行環境で 1Password 連携が阻害されることが事前に分かる場合、最初から制約のない実行環境で対象の `git` コマンドを実行する。

事前に判断できない場合は、通常の実行環境で試してよい。ただし、1Password SSH agent への接続失敗、署名失敗、承認プロンプト不達、SSH 認証失敗、socket 通信エラー、またはそれに類するエラーが出た場合は、同じ `git` コマンドを制約のない実行環境で再実行する。署名の失敗を切り分けるときは、「署名の実行経路」で経路を確かめてから判断する。

### 署名の実行経路

署名の実行経路は `gpg.ssh.program` の設定で変わる。署名の失敗の切り分けは、この確認から始める。

```sh
git config --get gpg.ssh.program
```

- 1Password の signer を使う場合(`op-ssh-sign` を指す): 署名は 1Password app が直接処理し、SSH agent を経由しない。`SSH_AUTH_SOCK` と `ssh-add -l` の結果は署名の可否と無関係であり、`ssh-add -l` を根拠に判断しない(`The agent has no identities.` が返っても、署名が失敗するとは限らない)。`SSH_AUTH_SOCK` を上書きする必要も無い。
- 標準の ssh-keygen を使う場合(未設定): 署名は `ssh-keygen -Y sign` が行う。署名鍵を 1Password に置く構成では、鍵を `SSH_AUTH_SOCK` が指す SSH agent から取るため、`ssh-add -l` で署名鍵が見えるかを確かめられる。`~/.ssh/config` の `IdentityAgent` は `ssh` の接続にだけ効き、`ssh-keygen -Y sign` には効かない。

## cloud session(Claude Code on the web)

cloud session では上記「1Password 連携が必要な操作」を適用しない(`doc/guidelines/cloud-session-guidelines.md`)。

- commit 署名と author は platform が管理する(`gpg.ssh.program` は platform の signer を指す)。1Password は関与しない。
- 作業ブランチは session 作成時に platform が決めたもの(`claude/<slug>` の形)に固定され、push はそのブランチにだけ許可される。別名のブランチを作らない。
- remote は HTTPS のまま platform の proxy が認証する。`main` への直接 push 禁止と PR 経由の原則は変わらない。
- tag の push はできない。署名付き tag の作成・push と `release` skill は cloud session では行わず、ローカルで行う。

## 通常の実行環境でよい操作

次のようなローカル参照・差分確認は、原則として通常の実行環境で実行してよい。

- `git status`
- `git diff`
- `git log`
- `git show`
- `git branch --show-current`
- `git rev-parse`

## 関連ルール

`gh pr create`、`gh pr view`、`gh run view` など、`git` コマンドではなく GitHub CLI(`gh`)を使う場合は `doc/guidelines/github-cli-guidelines.md` に従う。
