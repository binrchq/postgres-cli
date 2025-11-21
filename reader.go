package postgres

import (
	"io"
	
	"github.com/chzyer/readline"
)

// ReadWriteCloser wraps io.ReadWriter to add a no-op Close method
type ReadWriteCloser struct {
	io.ReadWriter
}

func (rwc *ReadWriteCloser) Close() error {
	return nil
}

// Reader 从终端读取输入（使用 readline 以支持SSH session）
type Reader struct {
	rl *readline.Instance
}

// NewReader 创建新的 Reader
func NewReader(term io.ReadWriter) *Reader {
	rwc := &ReadWriteCloser{term}
	rl, err := readline.NewEx(&readline.Config{
		Stdin:  rwc,
		Stdout: rwc,
		Prompt: "",
		InterruptPrompt: "^C",
		EOFPrompt: "exit",
	})
	if err != nil {
		panic(err)
	}
	return &Reader{rl: rl}
}

// ReadLine 读取一行输入
func (r *Reader) ReadLine() (string, error) {
	return r.rl.Readline()
}

// SetPrompt 设置提示符
func (r *Reader) SetPrompt(prompt string) {
	r.rl.SetPrompt(prompt)
}

// Close 关闭读取器
func (r *Reader) Close() error {
	return r.rl.Close()
}
