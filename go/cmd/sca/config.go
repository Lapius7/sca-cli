package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config はconfig.envの内容。PythonHelperDir以外はPython側(sca_realtime/config.py)と
// フィールド名・ファイルパスの意味を完全に一致させること。
type Config struct {
	SupabaseURL    string
	AnonKey        string
	ServiceRoleKey string
	Email          string
	PythonDir      string
}

func configDir() string {
	if v := os.Getenv("SCA_CONFIG_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "sca")
}

func configFilePath() string  { return filepath.Join(configDir(), "config.env") }
func sessionFilePath() string { return filepath.Join(configDir(), "session.json") }

func loadConfig() Config {
	cfg := Config{SupabaseURL: "https://supabase.lapius7.com"}
	values := map[string]string{}

	if data, err := os.ReadFile(configFilePath()); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if v != "" {
				values[k] = v
			}
		}
	}

	for _, k := range []string{"SUPABASE_URL", "ANON_KEY", "SERVICE_ROLE_KEY", "EMAIL", "PYTHON_DIR"} {
		if v := os.Getenv(k); v != "" {
			values[k] = v
		}
	}

	if v, ok := values["SUPABASE_URL"]; ok {
		cfg.SupabaseURL = v
	}
	cfg.AnonKey = values["ANON_KEY"]
	cfg.ServiceRoleKey = values["SERVICE_ROLE_KEY"]
	cfg.Email = values["EMAIL"]
	cfg.PythonDir = values["PYTHON_DIR"]
	return cfg
}

const configTemplate = `# sca (supabase-chat-app CLI) 設定ファイル
SUPABASE_URL=https://supabase.lapius7.com
ANON_KEY=
SERVICE_ROLE_KEY=
EMAIL=
# 別マシンでpythonディレクトリのパスが異なる場合だけ指定(未指定ならデフォルトの場所を使う)
# PYTHON_DIR=
`

func cmdInit() {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
	path := configFilePath()
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("既に存在します: %s\n", path)
		return
	}
	if err := os.WriteFile(path, []byte(configTemplate), 0600); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
	fmt.Printf("設定ファイルの雛形を作成しました: %s\n", path)
	fmt.Println("ANON_KEY / SERVICE_ROLE_KEY / EMAIL を書き込んでから `sca login` を実行してください。")
}

func requireLoginConfig(cfg Config) {
	if cfg.AnonKey == "" || cfg.ServiceRoleKey == "" {
		fmt.Fprintln(os.Stderr, "設定が不足しています: ANON_KEY, SERVICE_ROLE_KEY")
		fmt.Fprintf(os.Stderr, "`sca init` で %s の雛形を作成し、値を書き込んでください。\n", configFilePath())
		os.Exit(1)
	}
}
