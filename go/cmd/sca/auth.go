package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Session はsession.jsonの内容。Python側(sca_realtime/auth.py)が読む形式と完全一致させること。
type Session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Email        string `json:"email"`
}

func saveSession(s Session) error {
	if err := os.MkdirAll(configDir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionFilePath(), data, 0600)
}

func loadSession() (*Session, error) {
	data, err := os.ReadFile(sessionFilePath())
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func doLogout() {
	_ = os.Remove(sessionFilePath())
}

// マジックリンクのリダイレクト先を追跡するには、自動フォローを止めて
// Locationヘッダー(#access_token=...を含む)を直接読む必要がある。
var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// login はSERVICE_ROLE_KEYでマジックリンクを発行し、リダイレクトを追跡してセッションを取得する。
// ブラウザでのSSOハンドオフが使えないCLI向けの代替手段(実際にメールは送信されない)。
func login(cfg Config, email string) (*Session, error) {
	requireLoginConfig(cfg)
	if email == "" {
		email = cfg.Email
	}
	if email == "" {
		return nil, fmt.Errorf("ログインするメールアドレスが必要です(--email か config.envのEMAIL)")
	}

	base := strings.TrimRight(cfg.SupabaseURL, "/")
	body, _ := json.Marshal(map[string]string{
		"type":        "magiclink",
		"email":       email,
		"redirect_to": base + "/",
	})

	req, err := http.NewRequest(http.MethodPost, base+"/auth/v1/admin/generate_link", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", cfg.ServiceRoleKey)
	req.Header.Set("Authorization", "Bearer "+cfg.ServiceRoleKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("マジックリンクの生成に失敗しました(%d): %s", res.StatusCode, string(respBody))
	}

	var linkRes struct {
		ActionLink string `json:"action_link"`
	}
	if err := json.Unmarshal(respBody, &linkRes); err != nil || linkRes.ActionLink == "" {
		return nil, fmt.Errorf("action_linkの取得に失敗しました: %s", string(respBody))
	}

	redirectReq, err := http.NewRequest(http.MethodGet, linkRes.ActionLink, nil)
	if err != nil {
		return nil, err
	}
	redirectRes, err := noRedirectClient.Do(redirectReq)
	if err != nil {
		return nil, err
	}
	defer redirectRes.Body.Close()
	location := redirectRes.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("マジックリンクの追跡に失敗しました(リダイレクトが返ってきませんでした)")
	}

	u, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	fragValues, err := url.ParseQuery(u.Fragment)
	if err != nil {
		return nil, err
	}
	accessToken := fragValues.Get("access_token")
	refreshToken := fragValues.Get("refresh_token")
	if accessToken == "" || refreshToken == "" {
		return nil, fmt.Errorf("トークンの取得に失敗しました: %s", location)
	}

	session := Session{AccessToken: accessToken, RefreshToken: refreshToken, Email: email}
	if err := saveSession(session); err != nil {
		return nil, err
	}
	return &session, nil
}
