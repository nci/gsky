package metrics

import (
	"bytes"
	"encoding/json"
	"log"
)

type Logger interface {
	Log(info *MetricsInfo)
}

type StdoutLogger struct{}

func NewStdoutLogger() *StdoutLogger {
	return &StdoutLogger{}
}

func (l *StdoutLogger) Log(info *MetricsInfo) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(info)
	if err == nil {
		log.Print(buf.String())
	} else {
		log.Printf("StdoutLogger error: %v", err)
	}
}
