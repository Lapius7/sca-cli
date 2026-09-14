package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// spinner はインストーラー風の簡易プログレス表示。TTYでなければ静的な1行だけ出す
// (パイプ/リダイレクト時にちらつく制御文字が出力に混ざらないようにするため)。
type spinner struct {
	label  string
	suffix func() string // 毎tickで呼ばれ、末尾に追記する文字列を返す(バイト数表示など、無ければnil)
	stopCh chan struct{}
	wg     sync.WaitGroup
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newSpinner(label string) *spinner {
	return &spinner{label: label, stopCh: make(chan struct{})}
}

func (s *spinner) start() {
	if !colorEnabled {
		fmt.Printf("%s %s...\n", cyan("→"), s.label)
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				extra := ""
				if s.suffix != nil {
					if v := s.suffix(); v != "" {
						extra = " " + dim(v)
					}
				}
				fmt.Printf("\r\x1b[2K%s %s%s", cyan(spinnerFrames[i%len(spinnerFrames)]), s.label, extra)
				i++
			}
		}
	}()
}

// stop はスピナーを止め、finalMsgがあれば緑チェック付きの確定行に置き換える。
func (s *spinner) stop(finalMsg string) {
	if !colorEnabled {
		if finalMsg != "" {
			success(finalMsg)
		}
		return
	}
	close(s.stopCh)
	s.wg.Wait()
	fmt.Print("\r\x1b[2K")
	if finalMsg != "" {
		success(finalMsg)
	}
}

// countingReader はRead()経由で通過したバイト数を計測するラッパー(ダウンロード進捗表示用)。
type countingReader struct {
	r     interface{ Read([]byte) (int, error) }
	total int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	atomic.AddInt64(&c.total, int64(n))
	return n, err
}

func (c *countingReader) Total() int64 {
	return atomic.LoadInt64(&c.total)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
