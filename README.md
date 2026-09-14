# sca — supabase-chat-app CLIクライアント

`https://sandbox.lapius7.com/supabase-chat-app/` のチャット機能を、ブラウザを開かずターミナルの`sca`コマンドから使うためのCLI。

## 構成

- `go/` — 本体。ログイン(account.lapius7.comのSSOをブラウザ経由で利用)、ルームの一覧/作成/リネーム、REST API呼び出し全般を担当(Go標準ライブラリのみ、依存パッケージなし)
- `python/` — Realtime(オンライン一覧・対話チャット)専用の内部ヘルパー。Supabase RealtimeのWebSocket(Presence)プロトコルは公式Pythonパッケージ(`supabase`/`realtime`)に頼るため、この部分だけPythonで実装し、Go側からサブプロセスとして呼び出す

ユーザーが直接使うのは`go/`側のバイナリ(`sca`)だけで、Python側は`sca room who` / `sca room join`実行時に裏で自動的に呼ばれる。

## セットアップ

### インストーラースクリプトで(推奨)

```bash
curl -fsSL https://raw.githubusercontent.com/Lapius7/sca-cli/main/install.sh | sh
```

Go自体のインストール状況を確認し、ビルドの進行状況・インストール先・PATHの警告・次のステップまで1画面で
案内する(内部では`go install`を使うが、表示はこのスクリプトが整えている)。Realtime機能のPython環境もこの時点で自動的に準備される。

### `go install`で直接

```bash
go install github.com/lapius7/sca-cli/go/cmd/sca@latest
```

これでGitHubから直接ソースを取得してビルドし、`$(go env GOPATH)/bin/sca`にバイナリが入る
(`$(go env GOPATH)/bin`にPATHを通しておけばそのまま`sca`コマンドとして使える)。

### ソースから手元でビルドする場合

```bash
git clone https://github.com/lapius7/sca-cli
cd sca-cli/go
go build -o sca ./cmd/sca
sudo mv sca /usr/local/bin/
```

### 初期設定

```bash
sca login         # ブラウザでaccount.lapius7.com(Lapount)にログインし、自動的にセッションを取得する
```

これだけで使い始められる。設定ファイルを書く必要はなく、Realtime機能(`sca room who`/`join`)に
必要なPython環境もインストーラースクリプトが自動で準備する(`go install`で直接入れた場合や
準備前に初めて`sca room who`/`join`を実行した場合でも、その場で自動的にセットアップされる)。
CLIはsupabase.lapius7.comに直接繋がず、専用のリバースプロキシ
`https://sandbox.lapius7.com/supabase-chat-app/api/`
(実体は`web/sandbox.lapius7.com/supabase-chat-app/proxy/main.go`、REST/Auth/Realtime
WebSocketをすべて中継する)経由で通信する。ANON_KEYの付与はこのプロキシだけが行うため、
**CLI(Go/Pythonどちら側にも)はANON_KEYを一切持たない**。ANON_KEY自体はRLSで保護される
前提の非秘匿な値(Web版のJSバンドルにもそのまま入っている)なので厉密には埋め込んでも
問題ないが、CLIのソースコードに一切登場しない構成にすることで「念のため」のリスクも
無くしている。

`sca login`はローカルに一時HTTPサーバー(OSに選ばせたランダムなポート)を立て、まず
`oauth-connect-token`関数(action=mint、sca-proxy経由なのでANON_KEY不要)を呼んで
転送先(`http://127.0.0.1:<port>/callback`)に対応する使い捨てトークンを発行し、
`https://account.lapius7.com/oauth/authorize?token=<token>`をターミナルに表示する
(有効期限も併記する。ブラウザは自動で開かず、ユーザー自身がクリックまたはコピーして
開く)。account.lapius7.comのSSOハンドオフでログイン後、そのローカルサーバーにトークンが
自動的に返ってくる(`gh`/`aws`等のCLIと同じ方式)。こちらの`oauth-connect-token`呼び出し・
`/oauth/authorize`アクセスはsca-proxyを経由せずaccount.lapius7.com/supabase.lapius7.comに
直接アクセスする。`/oauth/authorize`は一般的なOAuth認可エンドポイントの見た目に合わせた
専用パスで、`sandbox.lapius7.com/supabase-chat-app`・`post.lapius7.com`・`md.lapius7.com`
などの既存サービスも同じ入口(と同じトークン発行の仈み)を共有している。実際の転送先URLは
URLに直接出ず、5分で失効する使い捨てトークンの向こう側にある。

ポートを固定(旧実装は`8765`固定)にしなかったのは、固定ポートだと第三者が事前に
同じポートを乗っ取っておき、フィッシングリンクでログインの確認画面だけ踏ませて
セッションを奧うことが理論上可能になるため(このCLIはOSSでポート番号も公開されている)。
`account.lapius7.com`側は`127.0.0.1`の任意ポートへのハンドオフだけを特例で許可しており
(`oauth-connect-token`関数・`sso-handoff`関数・`ssoRedirect.ts`)、他ドメインへの緩和は
一切行っていない。

別のSupabaseインスタンスに直接向けたい場合だけ、`~/.config/sca/config.env`に
`SUPABASE_URL`/`ANON_KEY`を書くか同名の環境変数を設定して上書きできる(この場合は
プロキシを経由しないので、自分のインスタンスのANON_KEYを指定する必要がある)。

## 使い方

```bash
sca whoami                          # ログイン中のユーザーを表示
sca room list                       # ルーム一覧
sca room create "雑談部屋"           # ルーム作成(作成後そのまま入室する)
sca room rename 雑談部屋 "雑談部屋2"  # リネーム(自分が作成したルームのみ)
sca room delete 雑談部屋2            # 削除(自分が作成したルームのみ、確認あり)
sca room who 雑談部屋2               # 今そのルームにいる人を表示
sca room join 雑談部屋2               # 入室して対話チャット開始
```

`<room_id_or_name>`にはルームIDでも名前でも指定できる(同名ルームが複数ある場合はIDでの指定を求められる)。

`sca room join`の対話セッション中は:
- 何か入力してEnterでメッセージ送信
- `/who` で現在のオンライン一覧
- `/invite` でこのルームへの招待方法(CLIコマンド・ブラウザ用URL)を表示
- `/quit`・`/leave`・Ctrl+C で退室(退室してもDBには何も残らない。Web版と同じ「切断するだけ」の挙動)

## 設計メモ

- `chat`スキーマには「入室中/メンバー」を表す永続テーブルは無い。入退室はRealtime Presenceチャンネル(`room-<roomId>`)への接続/切断だけで表現される、Web版と全く同じ仕組み
- ルームの削除は作成者のみ可能(`chat.chat_rooms`のRLSに`created_by = auth.uid()`のDELETEポリシーを追加済み)。削除するとメッセージも`ON DELETE CASCADE`で一緒に消える(FK制約側で設定)。CLI(`sca room delete`)・Web版どちらも実行前に確認を挿む。メッセージ自体の編集/削除は引き続き未実装(RLSポリシー未対応)
- `config.env`と`session.json`のパス・形式はGo/Python両方で共有しているので、片方だけ書き換えるとズレる点に注意(基本はGo側の`sca login`だけがセッションを書く)
- 複数マシンにこのリポジトリをsyncthingで同期している場合、Python venvの場所が既定(`python/.venv`)と異なるなら`config.env`に`PYTHON_DIR=...`を追記する
- プロキシ本体(`web/sandbox.lapius7.com/supabase-chat-app/proxy/`、このリポジトリの外側でVPS上にpm2常駐、nginxが`/supabase-chat-app/api/`パスをこれにproxy_pass)は`net/http/httputil.ReverseProxy`だけで実装したシンプルなリバースプロキシ。REST/Auth/Realtime WebSocketいずれもsupabase.lapius7.comへの単純な中継で、`apikey`ヘッダーを必ず上書きする以外は何もしない。**`apikey`のクエリパラメータ付与は`/realtime/v1/websocket`パスだけに限定すること**(PostgRESTは未知のクエリパラメータを列フィルタとして解釈するため、全パスに付与すると認証成功時にPGRST100エラーで壊れる。この不具合は無効なトークンでのテストでは表面化せず、実トークンで初めて発覚した)
- `oauth-connect-token`関数(`web/supabase.lapius7.com/volumes/functions/oauth-connect-token/`)は`public.oauth_connect_tokens`テーブル(token/redirect_to/expires_at/used_at、RLS有効・ポリシー無しでservice_role以外アクセス不可)にmint/resolveする使い捨てトークンの発行所。redirect_toの妥当性チェックはmint時にここで行う(`sso-handoff`関数も独立して同じチェックをしており、多層防御になっている)。このSupabaseインスタンスは全Edge Functionに`VERIFY_JWT=true`がグローバル設定されている(関数ごとの個別設定は非対応)ため、呼び出し側は`apikey`とは別に`Authorization: Bearer <ANON_KEY以上の有効なJWT>`も必須。sca-proxyはAuthorizationヘッダーが無い場合だけANON_KEYを補うので、CLIはここでも何も送らなくてよい
- `/oauth/authorize?token=...`のtokenが不正・期限切れ・使用済みの場合、`account.lapius7.com`はトップページへの無言リダイレクトではなく理由付きのエラー画面(`InvalidTokenScreen`)を表示する
- 確認画面(`AccountConfirm`)では、resolveで得た`expires_at`を1秒ごとに再計算する`ExpiryCountdown`でトークンの残り有効期限をライブ表示する(resolve自体はその場で使い捨てにするため、この表示は「本来あとどれくらいで無効になる想定だったか」を示すだけで、期限が来ても以降のログイン継続自体はブロックされない)
- Realtime機能のPython環境は`sca room who`/`join`実行時に自動セットアップされる(`ensurePythonReady`)。インストーラースクリプトもインストール直後に同じ処理を先回りして呼ぶので、通常はユーザーが手動でセットアップを意識する場面は無い(隠しコマンド`sca setup`で手動再実行も可能)。Python側は常にGitHubの`main`ブランチHEADから取得するが、以前は一度取得したら二度と更新しなかった(requirements.txtの有無しか見ていなかった)ため、`sca`本体だけタグ更新しても新しいPythonコードの変更(`/invite`追加等)が反映されない問題があった。ビルドに埋め込まれた`go install`のモジュールバージョン(`runtime/debug.ReadBuildInfo`)とキャッシュ側の記録を突き合わせ、ズレていたら再取得するようにしている
- `chat.py`の`on_message`は、送られてきたメッセージの`sender_id`が自分のuser_idと一致するかどうかだけで「CLIが送信したものだから表示しない(二重表示防止)」と判定していたが、これだと同じアカウントでブラウザからも送った場合にそのメッセージまで握り潰されてしまうバグがあった。現在はこのCLIセッションが実際に送信した内容だけを内容ベースの送信済みキュー(`ChatSession._pending_own`)で管理し、それ以外は自分のuser_idと一致していても表示する

## 既知の制約

- 対話セッション終了時、非同期タスクの後片付けに関する`Task was destroyed but it is pending!`という警告がstderrに出ることがある(cosmetic、動作・終了コードには影響しない)
- Presenceのキーはユーザー自身のuser_idなので、同じアカウントで複数のクライアント(CLI+ブラウザ等)を同時に開いても１人としてカウントされる(Web版と同じ仕様)
- `sca login`は`sca`を実行しているマシン自身でブラウザが開ける環境が前提(ローカルPC等)。SSH先のサーバー上でそのまま実行しても、ブラウザが手元のマシンで開いてもコールバックはSSH先に届かない。リモートで使う場合はターミナルに表示されるポート番号を確認して`ssh -L <そのポート>:localhost:<そのポート> ...`のようにポートフォワードすること(ポートは毎回ランダムなので、ログを見てから接続する必要がある)
