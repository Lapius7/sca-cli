package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// restRequest はPostgREST(/rest/v1/...)へのリクエストを共通化するヘルパー。
// schemaを指定すると Accept-Profile/Content-Profile ヘッダーでスキーマを切り替える
// (supabase-pyの`.schema("chat")`と同じ役割)。
func restRequest(cfg Config, session *Session, method, path, schema string, body interface{}) ([]byte, error) {
	base := strings.TrimRight(cfg.SupabaseURL, "/")

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", cfg.AnonKey)
	req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	if schema != "" {
		req.Header.Set("Accept-Profile", schema)
		req.Header.Set("Content-Profile", schema)
	}
	if method == http.MethodPost || method == http.MethodPatch || method == http.MethodDelete {
		req.Header.Set("Prefer", "return=representation")
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, wrapNetworkError(err)
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode == 401 {
		return nil, sessionExpiredError(respBody)
	}
	if res.StatusCode >= 300 {
		return nil, apiError(res.StatusCode, respBody)
	}
	return respBody, nil
}
