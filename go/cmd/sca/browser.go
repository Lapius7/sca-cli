package main

import (
	"os/exec"
	"runtime"
)

// openBrowser はOSのデフォルトブラウザでurlを開く。失敗してもエラーを返すだけで、
// 呼び出し側がURLをターミナルに表示してユーザーが手動で開けるようにする。
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
