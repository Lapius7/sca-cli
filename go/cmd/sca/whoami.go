package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// whoamiUser はGoTrueに現在のアクセストークンでユーザー情報を問い合わせ、
// (表示名, user_id)を返す。
func whoamiUser(cfg Config, session *Session) (string, string, error) {
	base := strings.TrimRight(cfg.SupabaseURL, "/")
	req, err := http.NewRequest(http.MethodGet, base+"/auth/v1/user", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("apikey", cfg.AnonKey)
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", "", fmt.Errorf("セッションが切れています。`sca login`でログインし直してください: %s", string(body))
	}

	var u struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", "", err
	}

	cache := map[string]string{}
	name := getDisplayName(cfg, session, u.ID, cache)
	return name, u.ID, nil
}
