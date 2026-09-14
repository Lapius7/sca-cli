package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "setup":
		// 通常は`sca room who`/`sca room join`実行時に自動で行われるので
		// 隠しコマンド扱い(printUsageには出さない)。手動での再実行・トラブル時用。
		if err := ensurePythonReady(loadConfig()); err != nil {
			fail(err)
		}
		success("Realtime機能の準備ができています。")
	case "login":
		cmdLogin(os.Args[2:])
	case "logout":
		doLogout()
		success("ログアウトしました。")
	case "whoami":
		cmdWhoami()
	case "room":
		cmdRoom(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Printf("%s - supabase-chat-app CLIクライアント\n\n", bold("sca"))
	fmt.Println(bold("Usage:"))
	rows := [][2]string{
		{"sca login", "ブラウザでログインする(account.lapius7.comのSSOを利用)"},
		{"sca logout", "ローカルのセッションを破棄する"},
		{"sca whoami", "ログイン中のユーザーを表示する"},
		{"sca room list", "ルーム一覧を表示する"},
		{"sca room create <name>", "ルームを作成する"},
		{"sca room rename <room_id_or_name> <new_name>", "ルーム名を変更する(作成者のみ)"},
		{"sca room who <room_id_or_name>", "ルームに今いる人を表示する"},
		{"sca room join <room_id_or_name>", "ルームに入って対話チャットを開始する"},
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(w, "  %s\t%s\n", cyan(r[0]), r[1])
	}
	w.Flush()
	fmt.Println()
	fmt.Println(dim("<room_id_or_name> にはルームIDまたは名前のどちらでも指定できます。"))
}

func requireSession() (Config, *Session) {
	cfg := loadConfig()
	session, err := loadSession()
	if err != nil {
		fail(fmt.Errorf("ログインしていません。先に `sca login` を実行してください"))
	}
	return cfg, session
}

func formatDate(iso string) string {
	if t, err := time.Parse(time.RFC3339Nano, iso); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return iso
}

func cmdLogin(args []string) {
	cfg := loadConfig()
	session, err := loginViaBrowser(cfg)
	if err != nil {
		fail(err)
	}
	success("ログインしました %s", dim("("+session.Email+")"))
}

func cmdWhoami() {
	cfg, session := requireSession()
	userID, err := whoamiUser(cfg, session)
	if err != nil {
		fail(err)
	}
	displayName, handle := getProfileFields(cfg, session, userID)

	fmt.Println(bold("ログイン中のユーザー"))
	if displayName != "" {
		fmt.Printf("  %s %s\n", dim("表示名:"), displayName)
	}
	if handle != "" {
		fmt.Printf("  %s %s\n", dim("ハンドル:"), handle)
	}
	if displayName == "" && handle == "" {
		fmt.Printf("  %s %s\n", dim("表示名:"), dim("(未設定)"))
	}
	if session.Email != "" {
		fmt.Printf("  %s %s\n", dim("メール:"), session.Email)
	}
	fmt.Printf("  %s %s\n", dim("ユーザーID:"), userID)
}

func cmdRoom(args []string) {
	if len(args) < 1 {
		printUsage()
		os.Exit(2)
	}
	cfg, session := requireSession()

	switch args[0] {
	case "list":
		roomList, err := listRooms(cfg, session)
		if err != nil {
			fail(err)
		}
		if len(roomList) == 0 {
			fmt.Println("ルームがありません。`sca room create <name>` で作成できます。")
			return
		}
		cache := map[string]string{}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", bold("NAME"), bold("ID"), bold("CREATED BY"), bold("CREATED AT"))
		for _, r := range roomList {
			creator := "?"
			if r.CreatedBy != nil {
				creator = getDisplayName(cfg, session, *r.CreatedBy, cache)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", bold(r.Name), dim(r.ID), creator, dim(formatDate(r.CreatedAt)))
		}
		w.Flush()

	case "create":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: sca room create <name>")
			os.Exit(2)
		}
		userID, err := whoamiUser(cfg, session)
		if err != nil {
			fail(err)
		}
		room, err := createRoom(cfg, session, args[1], userID)
		if err != nil {
			fail(err)
		}
		success("作成しました %s %s", bold(room.Name), dim(room.ID))
		// 作成したら普通そのまま使いたいはずなので、そのまま入室する
		if err := execRealtimeHelper(cfg, "join", room.ID, room.Name); err != nil {
			fail(err)
		}

	case "rename":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: sca room rename <room_id_or_name> <new_name>")
			os.Exit(2)
		}
		room, err := resolveRoom(cfg, session, args[1])
		if err != nil {
			fail(err)
		}
		updated, err := renameRoom(cfg, session, room.ID, args[2])
		if err != nil {
			fail(err)
		}
		success("リネームしました %s → %s", dim(room.Name), bold(updated.Name))

	case "who":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: sca room who <room_id_or_name>")
			os.Exit(2)
		}
		room, err := resolveRoom(cfg, session, args[1])
		if err != nil {
			fail(err)
		}
		if err := execRealtimeHelper(cfg, "who", room.ID, room.Name); err != nil {
			fail(err)
		}

	case "join":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: sca room join <room_id_or_name>")
			os.Exit(2)
		}
		room, err := resolveRoom(cfg, session, args[1])
		if err != nil {
			fail(err)
		}
		if err := execRealtimeHelper(cfg, "join", room.ID, room.Name); err != nil {
			fail(err)
		}

	default:
		printUsage()
		os.Exit(2)
	}
}
