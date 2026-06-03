# 0005 Cache Handling

- 状態: decided
- 作成日: 2026-06-02
- 最終更新日: 2026-06-02
- 関連: `../cache.md`

## 背景

`assets_manifest.json` や `metadata.json` は、HTML 出力後に利用者が直接参照する最終成果物ではなく、出力作成中に繰り返し参照される中間状態として扱える。

最終成果物と中間ファイルを同じ階層に置くと、利用者が保存・共有すべき範囲が分かりにくくなる。

## 候補

- `assets_manifest.json` や `metadata.json` を出力ディレクトリ直下に残す。
- 中間ファイルは `.cache/` 配下にまとめ、export 終了時に削除する。
- 中間ファイルをすべてメモリ上だけで扱い、ファイルとして保存しない。

## 検討内容

出力ディレクトリ直下に manifest や metadata を残す方式は単純だが、最終成果物と中間状態が混ざる。

メモリ上だけで扱う方式は成果物をきれいに保てるが、失敗時の調査や再実行時の再利用が難しい。

`.cache/` 配下にまとめる方式なら、実行中は再帰的に参照でき、export 終了時に削除できる。必要に応じて残したり、以前の cache を再利用する option も提供できる。

## 決定

`assets_manifest.json`、`metadata.json`、Slack API response や解決済み user / emoji / channel 情報などの中間状態は、出力データディレクトリ配下の `.cache/` に保存する。

通常動作では、export の成否に関係なく `.cache/` を削除する。原因調査や cache 再利用のために残したい場合は `--keep-cache` を指定する。

次の option を提供する方針とする。

- `--keep-cache`: export の成否に関係なく `.cache/` を削除せず残す。
- `--reuse-cache <path>`: 以前に保存した `.cache/` を読み込み、取得済み情報や asset manifest を再利用する。

`.cache/` には Slack token や secret を保存しない。

`--no-cache` は初期 option としては採用しない。cache を再利用したくない場合は `--reuse-cache` を指定しなければよく、cache を残したくない場合は `--keep-cache` を指定しなければよい。

## 理由

最終成果物は `index.html` と `assets/` に絞る方が、利用者が保存・共有すべき範囲を理解しやすい。

一方で、出力生成中は asset manifest、API response、user / emoji 解決結果などを繰り返し参照する必要がある。`.cache/` に集約すれば、実行中の効率、明示的な調査、再利用の余地を保てる。

## 影響

- 出力イメージでは `assets_manifest.json` や `metadata.json` を `.cache/` 配下に置く。
- `--keep-cache` を指定しない限り、成功/失敗に関係なく最終的な出力ディレクトリには `.cache/` が残らない。
- `.cache/` を残す場合、channel 名、user ID、message ID、file ID、元 URL などが含まれ得るため、共有や CI artifact 化に注意する。
- `--reuse-cache` 実装では、cache が同じ workspace、channel、取得条件に対応するか検証する必要がある。

## 後から見直す条件

- `.cache/` の再利用で stale data の問題が起きる。
- CI artifact として cache を残す運用が必要になる。
- cache に含まれる情報の秘匿性が高く、より厳密な保存制御が必要になる。
- 失敗時にも自動削除されることで、原因調査に必要な中間状態が失われる問題が大きくなる。
