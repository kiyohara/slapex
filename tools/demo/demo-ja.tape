# README 用ターミナルデモの VHS tape(Issue #115)。
#
# 素の `slapex` 実行を、tools/gensample の架空 fixture(エージェントラボ)を
# 配信する fake Slack API server に対して録画する。画面に映るデータはすべて
# 架空のもので、入力する token も架空値(echo なしのため表示されない)。
# 実行方法は bash tools/demo/record.sh(手順は doc/samples/README.md)。

Output assets/demo/slapex-demo-ja.gif

Set Shell bash
Set FontFamily "Noto Sans Mono CJK JP"
Set FontSize 20
Set Width 1080
Set Height 740
Set Padding 16
Set WindowBar Colorful
Set TypingSpeed 60ms

# 録画に映らない準備: PATH / TZ / 接続先 override を設定し、fixture server を
# 起動する。SLACK_TOKEN や plain 出力に倒す変数が混入していれば外す。
Hide
Type "export PATH=/vhs/tools/demo/bin:$PATH TZ=Asia/Tokyo SLAPEX_API_BASE_URL=http://127.0.0.1:8765/api/"
Enter
Type "unset SLACK_TOKEN NO_COLOR CI; gensample -serve -lang ja -asset-delay 250ms >/tmp/gensample-serve.log 2>&1 & sleep 1; cd; clear"
Enter
# clear の描画完了を待ってから可視キャプチャを再開する。待ちが無いと
# 最初のフレームに準備コマンドが映り込む(PR #116 レビュー指摘)。
Sleep 2s
Show

Sleep 1s
Type "slapex"
Sleep 500ms
Enter

# SLACK_TOKEN 入力プロンプト(入力は echo されない。値は架空)
Sleep 2.5s
Type@40ms "xoxp-0000000000-demo"
Sleep 500ms
Enter

# channel の対話選択(矢印キー + Enter)
Sleep 2.5s
Down
Sleep 1s
Enter

# 取得・ダウンロード・HTML 生成の進捗を完了表示まで録画する
Wait+Screen@120s /output:/
Sleep 4s
