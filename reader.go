package postgres

import (
	"fmt"
	"io"
)

// Reader 从终端读取输入
type Reader struct {
	term   io.Reader
	writer io.Writer
}

// NewReader 创建新的 Reader
func NewReader(term io.ReadWriter) *Reader {
	return &Reader{term: term, writer: term}
}

// ReadLine 读取一行输入
func (r *Reader) ReadLine() (string, error) {
	line := ""
	buf := make([]byte, 1)
	
	for {
		n, err := r.term.Read(buf)
		if err != nil {
			return "", err
		}
		
		if n > 0 {
			ch := buf[0]
			if ch == '\n' || ch == '\r' {
				if ch == '\r' {
					r.term.Read(buf) // skip \n
				}
				return line, nil
			} else if ch == 127 || ch == 8 { // Backspace
				if len(line) > 0 {
					line = line[:len(line)-1]
					fmt.Fprintf(r.writer, "\b \b")
				}
			} else if ch >= 32 && ch < 127 { // 可打印字符
				line += string(ch)
				fmt.Fprintf(r.writer, string(ch))
			}
		}
	}
}

