package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultSupabaseURL/defaultAnonKey はsupabase.lapius7.comの既定値。ANON_KEYはRLSで
// 保護される前提の公開キーであり(sandbox.lapius7.com/supabase-chat-app/のJSバンドルに
// そのまま埋め込まれているのと同じ機密度)、CLIに同梱しても問題ない。これにより
// 一般ユーザーはconfig.envを一切書かずに`sca login`だけで使い始められる。
const (
	defaultSupabaseURL = "https://supabase.lapius7.com"
	defaultAnonKey     = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYW5vbiIsImlzcyI6InN1cGFiYXNlIiwiaWF0IjoxNzg5MTYwNDMwLCJleHAiOjE5NDY4NDA0MzB9.bk3hxt1IXRa2UlACmQ3N1MzacqGnN7Od-gQcRpHe4Qs"
)

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
	cfg := Config{SupabaseURL: defaultSupabaseURL, AnonKey: defaultAnonKey}
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

// configTemplateはデフォルトのsupabase.lapius7.com以外に向ける場合だけ使う上書き用の雛形。
// 通常利用ではsca init/config.envは不要(sca loginだけで動く)。
const configTemplate = `# sca (supabase-chat-app CLI) 設定ファイル
# 通常は書き換え不要(既定でsupabase.lapius7.comに繋がる)。
# 別のSupabaseインスタンスに向けたい場合だけ以下を書き換える。
# SUPABASE_URL=
# ANON_KEY=
# 別マシンでpythonディレクトリのパスが異なる場合だけ指定(未指定ならデフォルトの場所を使う)
# PYTHON_DIR=
`

func cmdInit() {
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		fail(err)
	}
	path := configFilePath()
	if _, err := os.Stat(path); err == nil {
		warn("既に存在します: %s", path)
		return
	}
	if err := os.WriteFile(path, []byte(configTemplate), 0600); err != nil {
		fail(err)
	}
	success("設定ファイルの雛形を作成しました %s", dim(path))
	fmt.Printf("  %s 通常は書き換え不要です。そのまま %s を実行できます。\n", cyan("次:"), bold("sca login"))
}
