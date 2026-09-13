#!/usr/bin/env bash
# sca (supabase-chat-app CLI) のワンライナーインストーラー。
#
#   curl -fsSL https://raw.githubusercontent.com/Lapius7/cli-client_supabase-chat-app/main/install.sh | sh
#
set -euo pipefail

REPO="github.com/lapius7/cli-client_supabase-chat-app"
PKG="${REPO}/go/cmd/sca"

if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RESET=$'\033[0m'
  RED=$'\033[31m'; GREEN=$'\033[32m'; CYAN=$'\033[36m'; YELLOW=$'\033[33m'
else
  BOLD=""; DIM=""; RESET=""; RED=""; GREEN=""; CYAN=""; YELLOW=""
fi

info() { printf "%s→%s %s\n" "$CYAN" "$RESET" "$1"; }
ok()   { printf "%s✓%s %s\n" "$GREEN" "$RESET" "$1"; }
warn() { printf "%s!%s %s\n" "$YELLOW" "$RESET" "$1"; }
err()  { printf "%s✗%s %s\n" "$RED" "$RESET" "$1" >&2; }

printf "%ssca%s — supabase-chat-app CLI installer\n\n" "$BOLD" "$RESET"

if ! command -v go >/dev/null 2>&1; then
  err "Go が見つかりません。https://go.dev/dl/ からインストールしてから、もう一度実行してください。"
  exit 1
fi
GO_VERSION="$(go version | awk '{print $3}')"
ok "Go を検出しました ${DIM}${GO_VERSION}${RESET}"

info "sca をダウンロード・ビルド中 (${PKG}@latest)"
LOG="$(mktemp)"
if go install "${PKG}@latest" >"$LOG" 2>&1; then
  ok "ビルド完了"
else
  err "インストールに失敗しました"
  cat "$LOG" >&2
  rm -f "$LOG"
  exit 1
fi
rm -f "$LOG"

GOBIN="$(go env GOPATH)/bin"
BIN="${GOBIN}/sca"

if [ ! -x "$BIN" ]; then
  err "ビルドは成功しましたが、想定の場所にバイナリが見つかりません: $BIN"
  exit 1
fi
ok "インストール先: ${DIM}${BIN}${RESET}"

printf "\n"
info "Realtime機能(オンライン一覧・対話チャット)を準備中"
"$BIN" setup || warn "Realtime機能の準備に失敗しました(sca room who/join実行時に再度自動で試みます)"

case ":${PATH}:" in
  *":${GOBIN}:"*) ;;
  *)
    printf "\n"
    warn "PATHに ${GOBIN} が通っていません"
    printf "  ご使用のシェルの設定ファイル(~/.bashrc, ~/.zshrc 等)に以下を追記してください:\n"
    printf "  %sexport PATH=\"\$PATH:%s\"%s\n" "$DIM" "$GOBIN" "$RESET"
    ;;
esac

printf "\n%s🎉 sca のインストールが完了しました！%s\n\n" "$BOLD" "$RESET"
printf "次のステップ:\n"
printf "  %s1.%s %ssca login%s    ブラウザでログイン\n" "$BOLD" "$RESET" "$CYAN" "$RESET"
printf "\n%s詳細:%s https://github.com/Lapius7/cli-client_supabase-chat-app\n" "$DIM" "$RESET"
