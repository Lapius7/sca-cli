# sca — supabase-chat-app CLIクライアント

`https://sandbox.lapius7.com/supabase-chat-app/` のチャット機能を、ブラウザを開かずターミナルの`sca`コマンドから使うためのCLI。

## 構成

- `go/` — 本体。ログイン(account.lapius7.comのSSOをブラウザ経由で利用)、ルームの一覧/作成/リネーム、REST API呼び出し全般を担当(Go標準ライブラリのみ、依存パッケージなし)
- `python/` — Realtime(オンライン一覧・対話チャット)専用の内部ヘルパー。Supabase RealtimeのWebSocket(Presence)プロトコルは公式Pythonパッケージ(`supabase`/`realtime`)に頼るため、この部分だけPythonで実装し、Go側からサブプロセスとして呼び出す

ユーザーが直接使うのは`go/`側のバイナリ(`sca`)だけで、Python側は`sca room who` / `sca room join`実行時に裏で自動的に呼ばれる。

## セットアップ

### インストーラースクリプトで(推奨)

```bash
curl -fsSL https://raw.githubusercontent.com/Lapius7/cli-client_supabase-chat-app/main/install.sh | sh
```

Go自体のインストール状況を確認し、ビルドの進行状況・インストール先・PATHの警告・次のステップまで1画面で
案内する(内部では`go install`を使うが、表示はこのスクリプトが整えている)。

### `go install`で直接

```bash
go install github.com/lapius7/cli-client_supabase-chat-app/go/cmd/sca@latest
```

これでGitHubから直接ソースを取得してビルドし、`$(go env GOPATH)/bin/sca`にバイナリが入る
(`$(go env GOPATH)/bin`にPATHを通しておけばそのまま`sca`コマンドとして使える)。

### ソースから手元でビルドする場合

```bash
git clone https://github.com/lapius7/cli-client_supabase-chat-app
cd cli-client_supabase-chat-app/go
go build -o sca ./cmd/sca
sudo mv sca /usr/local/bin/
```

### 初期設定

```bash
sca setup         # Realtime機能に必要なPythonヘルパー一式をGitHubから取得してvenvを作成(初回のみ)
sca login         # ブラウザでaccount.lapius7.com(Lapount)にログインし、自動的にセッションを取得する
```

インストール後、設定ファイルを一切書かずに`sca login`だけで使い始められる
(`SUPABASE_URL`/`ANON_KEY`は既定でsupabase.lapius7.comを向くようバイナリに埋め込み済み。
`ANON_KEY`はRLSで保護される前提の公開キーで、そもそもWeb版のJSバンドルにもそのまま
入っているものなので同梱して問題ない。**SERVICE_ROLE_KEYのような秘密情報はCLI側に一切持たない**)。

`sca login`はローカルに一時HTTPサーバー(`127.0.0.1:8765`)を立ててブラウザを開き、
account.lapius7.comのSSOハンドオフでログイン後、そのローカルサーバーにトークンが
自動的に返ってくる(`gh`/`aws`等のCLIと同じ方式)。

別のSupabaseインスタンスに向けたい場合だけ`sca init`で雛形を作り、`SUPABASE_URL`/`ANON_KEY`を上書きできる。

`sca setup`は、Pythonヘルパー(`python/`)が手元に無ければ自動的にGitHubのtarballから
`python/`ディレクトリだけを取得して`~/.local/share/sca/python`に展開する
(`go install`でバイナリだけ入れた場合でも、リポジトリを別途cloneする必要はない)。

## 使い方

```bash
sca whoami                          # ログイン中のユーザーを表示
sca room list                       # ルーム一覧
sca room create "雑談部屋"           # ルーム作成
sca room rename 雑談部屋 "雑談部屋2"  # リネーム(自分が作成したルームのみ)
sca room who 雑談部屋2               # 今そのルームにいる人を表示
sca room join 雑談部屋2               # 入室して対話チャット開始
```

`<room>`にはルームIDでも名前でも指定できる(同名ルームが複数ある場合はIDでの指定を求められる)。

`sca room join`の対話セッション中は:
- 何か入力してEnterでメッセージ送信
- `/who` で現在のオンライン一覧
- `/quit`・`/leave`・Ctrl+C で退室(退室してもDBには何も残らない。Web版と同じ「切断するだけ」の挙動)

## 設計メモ

- `chat`スキーマには「入室中/メンバー」を表す永続テーブルは無い。入退室はRealtime Presenceチャンネル(`room-<roomId>`)への接続/切断だけで表現される、Web版と全く同じ仕組み
- ルームの削除・メッセージの編集/削除は現状のRLSポリシーが対応していないため未実装(将来必要になれば`account.lapius7.com/admin/rls-policy`でポリシー追加が先)
- `config.env`と`session.json`のパス・形式はGo/Python両方で共有しているので、片方だけ書き換えるとズレる点に注意(基本はGo側の`sca login`だけがセッションを書く)
- 複数マシンにこのリポジトリをsyncthingで同期している場合、Python venvの場所が既定(`python/.venv`)と異なるなら`config.env`に`PYTHON_DIR=...`を追記する

## 既知の制約

- 対話セッション終了時、非同期タスクの後片付けに関する`Task was destroyed but it is pending!`という警告がstderrに出ることがある(cosmetic、動作・終了コードには影響しない)
- Presenceのキーはユーザー自身のuser_idなので、同じアカウントで複数のクライアント(CLI+ブラウザ等)を同時に開いても１人としてカウントされる(Web版と同じ仕様)
- `sca login`は`sca`を実行しているマシン自身でブラウザが開ける環境が前提(ローカルPC等)。SSH先のサーバー上でそのまま実行しても、ブラウザが手元のマシンで開いてもコールバック(`127.0.0.1:8765`)はSSH先に届かない。リモートで使う場合は`ssh -L 8765:localhost:8765 ...`のようにポートフォワードすること
