// Package prompt は対話的な確認プロンプトを提供する。
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Confirm は question を out に表示し、in から 1 行読んで y/yes なら true を返す。
// 入力が EOF で終わった場合（パイプが空など）は拒否として扱う。
func Confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	if _, err := fmt.Fprintf(out, "%s [y/N]: ", question); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	if err == io.EOF && line == "" {
		// 改行されないまま終わるとプロンプトの後ろに続けて出力されるため改行しておく。
		_, _ = fmt.Fprintln(out)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}
