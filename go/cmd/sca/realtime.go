package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	githubRepo = "lapius7/cli-client_supabase-chat-app"
	githubRef  = "main"
)

// pythonDir はRealtime(Presence/購読)を担当するPythonヘルパーの置き場所。
// config.envの PYTHON_DIR で明示されていればそれを使い、無ければ
// ユーザーのローカルキャッシュ(~/.local/share/sca/python)を既定値にする
// (go installでバイナリだけ配布された場合、リポジトリのクローンが手元に無いため)。
func pythonDir(cfg Config) string {
	if cfg.PythonDir != "" {
		return cfg.PythonDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "sca", "python")
}

func pythonInterpreter(dir string) string {
	venvPython := filepath.Join(dir, ".venv", "bin", "python3")
	if _, err := os.Stat(venvPython); err == nil {
		return venvPython
	}
	return "python3" // venv未セットアップ時のフォールバック(依存パッケージが入っていれば動く)
}

// fetchPythonFromGitHub はGitHubのtarball(codeload)からリポジトリ全体を取得し、
// その中の`python/`サブディレクトリだけをtargetDirに展開する。
// (GitHubは単一サブディレクトリだけのダウンロードURLを提供していないため、
// 一度全体を取得してフィルタしながら展開する)
func fetchPythonFromGitHub(targetDir string) error {
	url := fmt.Sprintf("https://codeload.github.com/%s/tar.gz/refs/heads/%s", githubRepo, githubRef)
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("GitHubからの取得に失敗しました(%d): %s", res.StatusCode, url)
	}

	gz, err := gzip.NewReader(res.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	prefix := "" // 例: "cli-client_supabase-chat-app-main/python/"
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if prefix == "" {
			parts := strings.SplitN(hdr.Name, "/", 2)
			prefix = parts[0] + "/python/"
		}
		if !strings.HasPrefix(hdr.Name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(hdr.Name, prefix)
		if rel == "" {
			continue
		}
		found = true
		target := filepath.Join(targetDir, rel)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := hdr.FileInfo().Mode()
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	if !found {
		return fmt.Errorf("リポジトリ内に python/ ディレクトリが見つかりませんでした")
	}
	return nil
}

// cmdSetup はRealtime機能に必要なPythonヘルパー(未取得ならGitHubから取得)とvenvを用意する。
func cmdSetup() {
	cfg := loadConfig()
	dir := pythonDir(cfg)

	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err != nil {
		step("Pythonヘルパーが見つからないため、GitHubから取得します %s", dim(dir))
		if err := os.MkdirAll(dir, 0755); err != nil {
			fail(err)
		}
		if err := fetchPythonFromGitHub(dir); err != nil {
			fail(fmt.Errorf("取得に失敗しました: %w", err))
		}
		success("取得しました")
	}

	venvDir := filepath.Join(dir, ".venv")
	step("Python venvを準備しています %s", dim(venvDir))
	run := func(name string, args ...string) {
		c := exec.Command(name, args...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fail(fmt.Errorf("失敗しました(%s %v): %w", name, args, err))
		}
	}
	run("python3", "-m", "venv", venvDir)
	run(filepath.Join(venvDir, "bin", "pip"), "install", "-r", filepath.Join(dir, "requirements.txt"))
	success("完了しました。 %s / %s が使えるようになりました。", bold("sca room who"), bold("sca room join"))
}

// execRealtimeHelper はPythonヘルパー(sca_realtime)をサブプロセスとして起動し、
// 標準入出力をそのまま引き継ぐ(joinは対話セッションのため必須)。
func execRealtimeHelper(cfg Config, action, roomID, roomName string) error {
	dir := pythonDir(cfg)
	interpreter := pythonInterpreter(dir)

	cmd := exec.Command(interpreter, "-m", "sca_realtime", action, roomID, roomName)
	cmd.Dir = filepath.Join(dir, "src")
	cmd.Env = append(os.Environ(), "SCA_CONFIG_DIR="+configDir())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if _, statErr := os.Stat(interpreter); statErr != nil {
			return fmt.Errorf("Python環境が見つかりません。先に `sca setup` を実行してください: %w", err)
		}
		return err
	}
	return nil
}
