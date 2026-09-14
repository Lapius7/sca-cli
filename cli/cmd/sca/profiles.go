package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// getProfileFields は public.user_profiles から表示名・ハンドルをそのまま返す
// (未設定でも空文字のままにする。`sca whoami`のように全項目をきちんと見せたい場合用)。
func getProfileFields(cfg Config, session *Session, userID string) (displayName, handle string) {
	path := fmt.Sprintf("/rest/v1/user_profiles?user_id=eq.%s&select=display_name,handle", url.QueryEscape(userID))
	data, err := restRequest(cfg, session, http.MethodGet, path, "", nil)
	if err != nil {
		return "", ""
	}
	var rows []struct {
		DisplayName *string `json:"display_name"`
		Handle      *string `json:"handle"`
	}
	if json.Unmarshal(data, &rows) != nil || len(rows) == 0 {
		return "", ""
	}
	if rows[0].DisplayName != nil {
		displayName = *rows[0].DisplayName
	}
	if rows[0].Handle != nil {
		handle = *rows[0].Handle
	}
	return displayName, handle
}
