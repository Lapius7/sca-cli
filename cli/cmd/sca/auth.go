package main

import (
	"encoding/json"
	"io"
	"net/http"
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

// fetchEmail は取得したアクセストークンでGoTrueに問い合わせ、ユーザーのメールアドレスを取る
// (session.jsonの表示用フィールドを埋めるためだけに使う。ログイン処理自体には不要)。
func fetchEmail(cfg Config, accessToken string) string {
	base := strings.TrimRight(cfg.SupabaseURL, "/")
	req, err := http.NewRequest(http.MethodGet, base+"/auth/v1/user", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("apikey", cfg.AnonKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return ""
	}
	var u struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return ""
	}
	return u.Email
}
