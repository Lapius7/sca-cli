package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Profile は public.user_profiles の1行分(Lapountで設定したハンドル・表示名など)。
type Profile struct {
	DisplayName string
	Handle      string
	Locale      string
	Timezone    string
}

// getProfile は public.user_profiles から表示名・ハンドル・ロケール・タイムゾーンを
// そのまま返す(未設定でもゼロ値のままにする。`sca whoami`で全項目をきちんと
// 見せたい場合用)。
func getProfile(cfg Config, session *Session, userID string) Profile {
	path := fmt.Sprintf("/rest/v1/user_profiles?user_id=eq.%s&select=display_name,handle,locale,timezone", url.QueryEscape(userID))
	data, err := restRequest(cfg, session, http.MethodGet, path, "", nil)
	if err != nil {
		return Profile{}
	}
	var rows []struct {
		DisplayName *string `json:"display_name"`
		Handle      *string `json:"handle"`
		Locale      *string `json:"locale"`
		Timezone    *string `json:"timezone"`
	}
	if json.Unmarshal(data, &rows) != nil || len(rows) == 0 {
		return Profile{}
	}
	p := Profile{}
	if rows[0].DisplayName != nil {
		p.DisplayName = *rows[0].DisplayName
	}
	if rows[0].Handle != nil {
		p.Handle = *rows[0].Handle
	}
	if rows[0].Locale != nil {
		p.Locale = *rows[0].Locale
	}
	if rows[0].Timezone != nil {
		p.Timezone = *rows[0].Timezone
	}
	return p
}
