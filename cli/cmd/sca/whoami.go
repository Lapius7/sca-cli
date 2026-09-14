package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Identity は/auth/v1/userが返す連携プロバイダ1件分。
type Identity struct {
	Provider     string `json:"provider"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
	LastSignInAt string `json:"last_sign_in_at"`
}

// MFAFactor は/auth/v1/userが返す2段階認証(MFA) factor1件分。
type MFAFactor struct {
	Status           string `json:"status"`
	FactorType       string `json:"factor_type"`
	FriendlyName     string `json:"friendly_name"`
	CreatedAt        string `json:"created_at"`
	LastChallengedAt string `json:"last_challenged_at"`
}

// UserDetail は/auth/v1/user(自己ユーザー情報。admin権限不要)のレスポンス。
// `sca whoami`表示用に、Lapount管理画面(/admin/users/:id)が見せているのと
// 同水準の情報(連携プロバイダ・MFA状態・登録/最終ログイン日時)をここに集約する。
type UserDetail struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	EmailConfirmedAt string `json:"email_confirmed_at"`
	CreatedAt        string `json:"created_at"`
	LastSignInAt     string `json:"last_sign_in_at"`
	AppMetadata      struct {
		Provider  string   `json:"provider"`
		Providers []string `json:"providers"`
	} `json:"app_metadata"`
	UserMetadata struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
	} `json:"user_metadata"`
	Factors    []MFAFactor `json:"factors"`
	Identities []Identity  `json:"identities"`
}

// fetchAuthUser はGoTrueの自己ユーザー情報(/auth/v1/user)への問い合わせを共通化する。
// 401(アクセストークン失効)の場合はRefreshTokenで1回だけ更新してから再試行する
// (restRequestと同じ理由。以前はwhoami専用に同じ再試行ロジックが重複していた)。
func fetchAuthUser(cfg Config, session *Session) ([]byte, error) {
	body, status, err := doFetchAuthUser(cfg, session)
	if err != nil {
		return nil, err
	}
	if status == 401 {
		if refreshErr := refreshAccessToken(cfg, session); refreshErr != nil {
			return nil, refreshErr
		}
		body, status, err = doFetchAuthUser(cfg, session)
		if err != nil {
			return nil, err
		}
	}
	if status >= 300 {
		return nil, sessionExpiredError(body)
	}
	return body, nil
}

func doFetchAuthUser(cfg Config, session *Session) ([]byte, int, error) {
	base := strings.TrimRight(cfg.SupabaseURL, "/")
	req, err := http.NewRequest(http.MethodGet, base+"/auth/v1/user", nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("apikey", cfg.AnonKey)
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, wrapNetworkError(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return body, res.StatusCode, nil
}

// whoamiUser はuser_idだけが欲しい呼び出し元(room系コマンド)向けの軽量ラッパー。
func whoamiUser(cfg Config, session *Session) (string, error) {
	body, err := fetchAuthUser(cfg, session)
	if err != nil {
		return "", err
	}
	var u struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	return u.ID, nil
}

// fetchUserDetail は`sca whoami`/ログイン後サマリー用に、連携プロバイダ・MFA状態を
// 含むフル情報を取得する。
func fetchUserDetail(cfg Config, session *Session) (*UserDetail, error) {
	body, err := fetchAuthUser(cfg, session)
	if err != nil {
		return nil, err
	}
	var u UserDetail
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func cmdWhoami() {
	cfg, session := requireSession()
	detail, err := fetchUserDetail(cfg, session)
	if err != nil {
		fail(err)
	}
	profile := getProfile(cfg, session, detail.ID)
	printWhoamiFull(detail, profile)
}
