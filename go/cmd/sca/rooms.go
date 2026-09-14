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

// listRooms は自分が作成したルームだけを返す。chat_rooms は全認証済みユーザーが
SELECT可能なRLSのため、絞り込み無しで一覧すると他人が作ったルームの存在・名前・IDまで
見えてしまう(名前を知られただけで入室されるリスクにつながる)。一覧はあくまで
「自分のルームの管理画面」とし、他人のルームへは`/invite`で渡されたIDでのみ入れるようにする。
func listRooms(cfg Config, session *Session, ownerID string) ([]Room, error) {
	path := fmt.Sprintf("/rest/v1/chat_rooms?select=*&created_by=eq.%s&order=created_at.desc", url.QueryEscape(ownerID))
	data, err := restRequest(cfg, session, http.MethodGet, path, "chat", nil)
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

func deleteRoom(cfg Config, session *Session, roomID string) error {
	path := fmt.Sprintf("/rest/v1/chat_rooms?id=eq.%s", url.QueryEscape(roomID))
	data, err := restRequest(cfg, session, http.MethodDelete, path, "chat", nil)
	if err != nil {
		return err
	}
	var rooms []Room
	if err := json.Unmarshal(data, &rooms); err != nil || len(rooms) == 0 {
		return errors.New("削除に失敗しました(自分が作成したルームでない可能性があります)")
	}
	return nil
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

// resolveRoom はルームIDでのみ解決する。以前は名前でも検索できたが、chat_rooms は
// 全認証済みユーザーがSELECT可能なRLSのため、名前検索を許すと他人のルーム名を
// 適当に打っただけで存在確認・IDの割り出し・入室ができてしまう問題があった。
// 自分のルームは`sca room list`(自分の作成分のみ)、他人のルームは`/invite`で
// 渡されたIDでのみ参照できるようにし、ID以外の入力は明確にエラーにする。
func resolveRoom(cfg Config, session *Session, ref string) (*Room, error) {
	if !isUUID(ref) {
		return nil, fmt.Errorf("ルームIDを指定してください(名前では入室できません): %q\n`sca room list`(自分のルーム)または招待されたIDを確認してください", ref)
	}

	path := fmt.Sprintf("/rest/v1/chat_rooms?id=eq.%s&select=*", url.QueryEscape(ref))
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
	return &rooms[0], nil
}
