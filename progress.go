package main

import (
	"bufio"
	"fmt"
	"io"
)

const progressByteLimit = 256 * 1024

func NewProgressReader(r io.Reader, progress *bufio.Writer) *ProgressReader {
	return &ProgressReader{
		r:        r,
		progress: progress,
	}
}

type ProgressReader struct {
	r        io.Reader
	progress *bufio.Writer

	n         int64
	lastPrint int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.n += int64(n)
	if pr.n-pr.lastPrint > progressByteLimit {
		fmt.Fprintf(pr.progress, "PROGRESS %d\n", pr.n)
		pr.progress.Flush()
		pr.lastPrint = pr.n
	}
	return n, err
}
