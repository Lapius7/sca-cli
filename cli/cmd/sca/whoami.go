package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// whoamiUser はGoTrueに現在のアクセストークンでユーザー情報を問い合わせ、user_idを返す。
func whoamiUser(cfg Config, session *Session) (string, error) {
	id, status, body, err := fetchWhoamiOnce(cfg, session)
	if err != nil {
		return "", err
	}
	if status == 401 {
		// restRequestと同じ理由(アクセストークン失効)。RefreshTokenでの更新を試してから
		// 1回だけやり直す
		if refreshErr := refreshAccessToken(cfg, session); refreshErr != nil {
			return "", refreshErr
		}
		id, status, body, err = fetchWhoamiOnce(cfg, session)
		if err != nil {
			return "", err
		}
	}
	if status >= 300 {
		return "", sessionExpiredError(body)
	}
	return id, nil
}

func fetchWhoamiOnce(cfg Config, session *Session) (string, int, []byte, error) {
	base := strings.TrimRight(cfg.SupabaseURL, "/")
	req, err := http.NewRequest(http.MethodGet, base+"/auth/v1/user", nil)
	if err != nil {
		return "", 0, nil, err
	}
	req.Header.Set("apikey", cfg.AnonKey)
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, nil, wrapNetworkError(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", res.StatusCode, body, nil
	}

	var u struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", res.StatusCode, body, err
	}
	return u.ID, res.StatusCode, body, nil
}
