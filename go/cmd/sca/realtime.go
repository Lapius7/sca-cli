package main

import (
	"archive/tar"
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

	// GitHubのcodeloadタルボールは "<repo>-<ref>/" というディレクトリの下に全ファイルを置く。
	// 先頭のtarエントリから動的に推測すると、GitHubが差し込む pax_global_header
	// (typeflag='g'、パスに"/"を含まない特殊エントリ)を誤って掴んでしまうため、
	// リポジトリ名から直接プレフィックスを組み立てる。
	repoBase := githubRepo[strings.LastIndex(githubRepo, "/")+1:]
	prefix := fmt.Sprintf("%s-%s/python/", repoBase, githubRef)
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
		return fmt.Errorf("リポジトリ内に python/ ディレクトリが見つかりませんでした")
	}
	return nil
}

// binaryVersion は `go install .../sca@latest` でビルドされた場合に埋め込まれる
// モジュールバージョン(例: "v0.3.2")を返す。ソースから直接ビルドした場合など
// バージョン情報が無い場合は空文字列を返す。
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

// ensurePythonReady はRealtime機能に必要なPythonヘルパー(未取得ならGitHubから取得)と
// venvを、無ければ用意する。既に用意済みなら何も表示せず即座に戻る。
// `sca room who`/`sca room join`実行時に自動で呼ばれるほか、install.shからも
// インストール直後に呼ばれる(ユーザーが別コマンドを意識する必要をなくすため)。
//
// go/がタグ更新されて`sca`本体だけ新しくなっても、以前は一度取得したPython側を
// 二度と更新しなかった(requirements.txtの有無しか見ていなかった)ため、新しい
// Pythonコードの変更(例: /inviteコマンド追加)がキャッシュ済み環境に反映されない
// 問題があった。ビルドに埋め込まれたモジュールバージョンとキャッシュ側に記録した
// バージョンを突き合わせ、ズレていたら再取得する。
func ensurePythonReady(cfg Config) error {
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
		return nil
	}

	fmt.Printf("%s Realtime機能を準備中です\n\n", cyan("→"))

	if needFetch {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := fetchPythonFromGitHub(dir); err != nil {
			return fmt.Errorf("Pythonヘルパーの取得に失敗しました: %w", err)
		}
		if v := binaryVersion(); v != "" {
			_ = os.WriteFile(versionFile, []byte(v), 0644)
		}
		needVenv = true // ディレクトリごと作り直したのでvenvも作り直す
	}

	sp := newSpinner("Python仮想環境を作成中")
	sp.start()
	if err := exec.Command("python3", "-m", "venv", venvDir).Run(); err != nil {
		sp.stop("")
		return fmt.Errorf("venvの作成に失敗しました(python3コマンドが必要です): %w", err)
	}
	sp.stop(fmt.Sprintf("venvを作成 %s", dim(venvDir)))

	pipCmd := exec.Command(pipPath, "install", "-r", filepath.Join(dir, "requirements.txt"))
	pipCmd.Stdout = os.Stdout
	pipCmd.Stderr = os.Stderr
	if err := pipCmd.Run(); err != nil {
		return fmt.Errorf("pip installに失敗しました: %w", err)
	}

	fmt.Println()
	success("Realtime機能のセットアップ完了")
	return nil
}

// execRealtimeHelper はPythonヘルパー(sca_realtime)をサブプロセスとして起動し、
// 標準入出力をそのまま引き継ぐ(joinは対話セッションのため必須)。
func execRealtimeHelper(cfg Config, action, roomID, roomName string) error {
	if err := ensurePythonReady(cfg); err != nil {
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

	// GoとPythonの子プロセスは同じフォアグラウンドプロセスグループにいるため、
	// ユーザーがCtrl+C(SIGINT)を押すと両方に同時に届く。Go側のデフォルト挙動
	// (即終了)のままだと、Python側が退室処理(Realtime切断・非同期タスクの
	// キャンセル)を終える前にGoプロセスだけ先に終了し、シェルのプロンプトが
	// 戻った後にPython側の出力が遅れて表示される、という見た目になっていた。
	// signal.Notifyで受信をこちらに引き取り、Pythonの終了(cmd.Run()の完了)
	// までGo側は生き続けるようにする。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	return cmd.Run()
}
