package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
)

const (
	githubRepo = "lapius7/sca-cli"
	githubRef  = "main"
)

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
	return "python3"
}

// fetchPythonFromGitHub はGitHubのtarball(codeload)からリポジトリ全体を取得し、
// その中の`realtime/`サブディレクトリだけをtargetDirに展開する。
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

	counter := &countingReader{r: res.Body}
	sp := newSpinner("GitHubからダウンロード中")
	sp.suffix = func() string { return formatBytes(counter.Total()) }
	sp.start()
	defer func() { sp.stop("ダウンロード完了 " + dim(formatBytes(counter.Total()))) }()

	gz, err := gzip.NewReader(counter)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	repoBase := githubRepo[strings.LastIndex(githubRepo, "/")+1:]
	prefix := fmt.Sprintf("%s-%s/realtime/", repoBase, githubRef)
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
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
		return fmt.Errorf("リポジトリ内に realtime/ ディレクトリが見つかりませんでした")
	}
	return nil
}

func binaryVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if bi.Main.Version == "" || bi.Main.Version == "(devel)" {
		return ""
	}
	return bi.Main.Version
}

func ensurePythonReady(cfg Config) (bool, error) {
	dir := pythonDir(cfg)
	venvDir := filepath.Join(dir, ".venv")
	pipPath := filepath.Join(venvDir, "bin", "pip")
	versionFile := filepath.Join(dir, ".fetched-version")

	needFetch := false
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err != nil {
		needFetch = true
	}
	if v := binaryVersion(); v != "" {
		fetched, _ := os.ReadFile(versionFile)
		if string(fetched) != v {
			needFetch = true
		}
	}
	needVenv := false
	if _, err := os.Stat(pipPath); err != nil {
		needVenv = true
	}
	if !needFetch && !needVenv {
		return false, nil
	}

	fmt.Printf("%s Realtime機能を準備中です\n\n", cyan("→"))

	if needFetch {
		if err := os.RemoveAll(dir); err != nil {
			return false, err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return false, err
		}
		if err := fetchPythonFromGitHub(dir); err != nil {
			return false, fmt.Errorf("Pythonヘルパーの取得に失敗しました: %w", err)
		}
		if v := binaryVersion(); v != "" {
			_ = os.WriteFile(versionFile, []byte(v), 0644)
		}
		needVenv = true
	}

	sp := newSpinner("Python仮想環境を作成中")
	sp.start()
	if err := exec.Command("python3", "-m", "venv", venvDir).Run(); err != nil {
		sp.stop("")
		return false, fmt.Errorf("venvの作成に失敗しました(python3コマンドが必要です): %w", err)
	}
	sp.stop(fmt.Sprintf("venvを作成 %s", dim(venvDir)))

	sp = newSpinner("依存パッケージをインストール中")
	sp.start()
	var pipOut bytes.Buffer
	pipCmd := exec.Command(pipPath, "install", "-q", "-r", filepath.Join(dir, "requirements.txt"))
	pipCmd.Stdout = &pipOut
	pipCmd.Stderr = &pipOut
	if err := pipCmd.Run(); err != nil {
		sp.stop("")
		fmt.Fprintln(os.Stderr, pipOut.String())
		return false, fmt.Errorf("pip installに失敗しました: %w", err)
	}
	sp.stop("依存パッケージをインストール")

	fmt.Println()
	success("Realtime機能のセットアップ完了")
	return true, nil
}

func execRealtimeHelper(cfg Config, action, roomID, roomName string) error {
	if _, err := ensurePythonReady(cfg); err != nil {
		return err
	}
	dir := pythonDir(cfg)
	interpreter := pythonInterpreter(dir)

	cmd := exec.Command(interpreter, "-m", "sca_realtime", action, roomID, roomName)
	cmd.Dir = filepath.Join(dir, "src")
	cmd.Env = append(os.Environ(), "SCA_CONFIG_DIR="+configDir())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	return cmd.Run()
}
