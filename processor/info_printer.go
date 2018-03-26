package processor

import (
	"io"
)

type JSONPrinter struct {
	In    chan []byte
	Out   chan struct{}
	File  io.Writer
	Error chan error
}

func NewJSONPrinter(file io.Writer, errChan chan error) *JSONPrinter {
	return &JSONPrinter{
		In:    make(chan []byte, 100),
		Out:   make(chan struct{}),
		File:  file,
		Error: errChan,
	}
}

func (jp *JSONPrinter) Run() {
	defer close(jp.Out)

	for line := range jp.In {
		jp.File.Write(line)
	}

}
