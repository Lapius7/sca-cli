package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

type Room struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	CreatedBy *string `json:"created_by"`
	CreatedAt string  `json:"created_at"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool { return uuidPattern.MatchString(s) }

func listRooms(cfg Config, session *Session) ([]Room, error) {
	data, err := restRequest(cfg, session, http.MethodGet, "/rest/v1/chat_rooms?select=*&order=created_at.desc", "chat", nil)
	if err != nil {
		return nil, err
	}
	var rooms []Room
	if err := json.Unmarshal(data, &rooms); err != nil {
		return nil, err
	}
	return rooms, nil
}

func createRoom(cfg Config, session *Session, name, ownerID string) (*Room, error) {
	data, err := restRequest(cfg, session, http.MethodPost, "/rest/v1/chat_rooms", "chat",
		map[string]string{"name": name, "created_by": ownerID})
	if err != nil {
		return nil, err
	}
	var rooms []Room
	if err := json.Unmarshal(data, &rooms); err != nil || len(rooms) == 0 {
		return nil, errors.New("ルームの作成に失敗しました")
	}
	return &rooms[0], nil
}

func renameRoom(cfg Config, session *Session, roomID, newName string) (*Room, error) {
	path := fmt.Sprintf("/rest/v1/chat_rooms?id=eq.%s", url.QueryEscape(roomID))
	data, err := restRequest(cfg, session, http.MethodPatch, path, "chat", map[string]string{"name": newName})
	if err != nil {
		return nil, err
	}
	var rooms []Room
	if err := json.Unmarshal(data, &rooms); err != nil || len(rooms) == 0 {
		return nil, errors.New("リネームに失敗しました(自分が作成したルームでない可能性があります)")
	}
	return &rooms[0], nil
}

// resolveRoom はUUIDならそのままIDとして、そうでなければ名前で検索して1件に絞り込む。
func resolveRoom(cfg Config, session *Session, ref string) (*Room, error) {
	var path string
	if isUUID(ref) {
		path = fmt.Sprintf("/rest/v1/chat_rooms?id=eq.%s&select=*", url.QueryEscape(ref))
	} else {
		path = fmt.Sprintf("/rest/v1/chat_rooms?name=eq.%s&select=*", url.QueryEscape(ref))
	}

	data, err := restRequest(cfg, session, http.MethodGet, path, "chat", nil)
	if err != nil {
		return nil, err
	}
	var rooms []Room
	if err := json.Unmarshal(data, &rooms); err != nil {
		return nil, err
	}
	if len(rooms) == 0 {
		return nil, fmt.Errorf("ルームが見つかりません: %s", ref)
	}
	if len(rooms) > 1 {
		msg := "同じ名前のルームが複数あります。IDで指定してください:\n"
		for _, r := range rooms {
			msg += fmt.Sprintf("  %s  %s\n", r.ID, r.Name)
		}
		return nil, errors.New(msg)
	}
	return &rooms[0], nil
}
