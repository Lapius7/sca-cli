package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	accountURL   = "https://account.lapius7.com"
	callbackPort = 8765
	loginTimeout = 3 * time.Minute
)

// callbackHTML はブラウザに一瞬だけ表示するページ。GoTrueはトークンをURL fragment
// (#access_token=...)で返すが、fragmentはサーバーに送られてこないため、
// このページのJSでlocation.hashを読み取り、ローカルサーバーにPOSTし直す。
const callbackHTML = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>sca ログイン</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f172a;color:#e2e8f0}
.box{text-align:center}h1{font-size:1.25rem}</style></head>
<body><div class="box"><h1 id="msg">処理中...</h1></div>
<script>
(function () {
  var params = new URLSearchParams(location.hash.replace(/^#/, ""));
  var accessToken = params.get("access_token");
  var refreshToken = params.get("refresh_token");
  var msg = document.getElementById("msg");
  if (!accessToken || !refreshToken) {
    msg.textContent = "ログインに失敗しました。このタブを閉じてCLIを確認してください。";
    fetch("/callback/complete", {
      method: "POST", headers: {"Content-Type": "application/json"},
      body: JSON.stringify({ error: "no_token" }),
    });
    return;
  }
  fetch("/callback/complete", {
    method: "POST", headers: {"Content-Type": "application/json"},
    body: JSON.stringify({ access_token: accessToken, refresh_token: refreshToken }),
  }).then(function () {
    msg.textContent = "ログインが完了しました。このタブを閉じてください。";
  }).catch(function () {
    msg.textContent = "CLIへの通知に失敗しました。ターミナルを確認してください。";
  });
})();
</script></body></html>`

type callbackResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
}

// loginViaBrowser はaccount.lapius7.com(Lapount)のSSOハンドオフを使い、
// ローカルにコールバック待受サーバーを立ててブラウザ経由でログインする
// (gh/aws等のCLIと同じ方式)。SERVICE_ROLE_KEYをCLI側に置く必要がない。
func loginViaBrowser(cfg Config) (*Session, error) {
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(callbackHTML))
	})
	mux.HandleFunc("/callback/complete", func(w http.ResponseWriter, r *http.Request) {
		var res callbackResult
		_ = json.NewDecoder(r.Body).Decode(&res)
		w.WriteHeader(http.StatusOK)
		select {
		case resultCh <- res:
		default:
		}
	})

	addr := fmt.Sprintf("127.0.0.1:%d", callbackPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ローカルサーバーの起動に失敗しました(ポート%dが使用中かもしれません): %w", callbackPort, err)
	}
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	redirectTo := fmt.Sprintf("http://127.0.0.1:%d/callback", callbackPort)
	authURL := strings.TrimRight(accountURL, "/") + "/oauth/authorize?redirect_to=" + url.QueryEscape(redirectTo)

	fmt.Printf("%s ブラウザでログインページを開きます:\n  %s\n", cyan("→"), authURL)
	if err := openBrowser(authURL); err != nil {
		warn("ブラウザを自動で開けませんでした。上記URLを手動で開いてください。")
	}

	select {
	case res := <-resultCh:
		if res.Error != "" || res.AccessToken == "" || res.RefreshToken == "" {
			return nil, fmt.Errorf("ログインに失敗しました(トークンを受信できませんでした)")
		}
		email := fetchEmail(cfg, res.AccessToken)
		session := Session{AccessToken: res.AccessToken, RefreshToken: res.RefreshToken, Email: email}
		if err := saveSession(session); err != nil {
			return nil, err
		}
		return &session, nil
	case <-time.After(loginTimeout):
		return nil, fmt.Errorf("タイムアウトしました(%s以内にブラウザでのログインが完了しませんでした)", loginTimeout)
	}
}
