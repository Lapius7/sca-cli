package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// defaultSupabaseURL はCLI専用のリバースプロキシ(sandbox.lapius7.com/supabase-chat-app/api/、
// 実体はweb/sandbox.lapius7.com/supabase-chat-app/proxy/main.go)を指す。このプロキシが
// supabase.lapius7.comへの全リクエストにANON_KEYを付与してから中継するため、CLI自体は
// ANON_KEYを一切持たない(ソースコードにも実行時の設定にも一度も登場しない)。
// これにより一般ユーザーはconfig.envを一切書かずに`sca login`だけで使い始められる。
const defaultSupabaseURL = "https://sandbox.lapius7.com/supabase-chat-app/api"

// Config はconfig.envの内容。PythonHelperDir以外はPython側(sca_realtime/config.py)と
// フィールド名・ファイルパスの意味を完全一致させること。
type Config struct {
	SupabaseURL string
	AnonKey     string
	PythonDir   string
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
	cfg := Config{SupabaseURL: defaultSupabaseURL}
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

	for _, k := range []string{"SUPABASE_URL", "ANON_KEY", "PYTHON_DIR"} {
		if v := os.Getenv(k); v != "" {
			values[k] = v
		}
	}

	if v, ok := values["SUPABASE_URL"]; ok {
		cfg.SupabaseURL = v
	}
	if v, ok := values["ANON_KEY"]; ok {
		cfg.AnonKey = v
	}
	cfg.PythonDir = values["PYTHON_DIR"]
	return cfg
}

// config.envは通常のユーザーには一切不要(sca loginだけで動く)。自前のSupabase
// インスタンスに直接向けたい上級者向けの上書き手段としてのみ残しており、
// 必要な場合は自分で ~/.config/sca/config.env (SUPABASE_URL=.../ANON_KEY=...) を
// 作成するか、同名の環境変数を設定する(専用の雛形生成コマンドはあえて用意しない)。
