package breaking

import (
	"fmt"
	"io"
	"strings"
)

type TextDiffReport struct {
	w      io.Writer
	prefix string
}

func (w TextDiffReport) Write(d []byte) (int, error) {
	toWrite := fmt.Sprintf("%s%s\n", w.prefix, strings.TrimSpace(strings.ReplaceAll(string(d), "\n", "\n"+w.prefix)))
	return w.w.Write([]byte(toWrite))
}
