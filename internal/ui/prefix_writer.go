package ui

import (
	"bytes"
	"io"
)

// NewLinePrefixWriter wraps w with a writer that prefixes every output line.
// The prefix is inserted at the beginning of each line, including blank lines,
// so callers can preserve indentation without piping through line-oriented
// filters that would make stdout/stderr non-terminals.
func NewLinePrefixWriter(w io.Writer, prefix string) io.Writer {
	if prefix == "" {
		return w
	}
	return &linePrefixWriter{
		dst:         w,
		prefix:      []byte(prefix),
		atLineStart: true,
	}
}

type linePrefixWriter struct {
	dst         io.Writer
	prefix      []byte
	atLineStart bool
}

func (w *linePrefixWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		if w.atLineStart {
			if _, err := w.dst.Write(w.prefix); err != nil {
				return written, err
			}
			w.atLineStart = false
		}

		newline := bytes.IndexByte(p, '\n')
		if newline < 0 {
			n, err := w.dst.Write(p)
			written += n
			return written, err
		}

		n, err := w.dst.Write(p[:newline+1])
		written += n
		if err != nil {
			return written, err
		}
		p = p[newline+1:]
		w.atLineStart = true
	}
	return written, nil
}
