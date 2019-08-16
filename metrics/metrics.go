package metrics

import (
	"bytes"
	"encoding/json"
	"time"
)

type IndexerInfo struct {
	Duration     time.Duration `json:"duration"`
	Query        string        `json:"query"`
	Geometry     string        `json:"geometry"`
	GeometryArea float64       `json:"geometry_area"`
	NumFiles     int           `json:"num_files"`
	NumGranules  int           `json:"num_granules"`
}

type RPCInfo struct {
	Duration         time.Duration `json:"duration"`
	NumTiledGranules int           `json:"num_tiled_granules"`
}

type MetricsInfo struct {
	ReqTime     string        `json:"req_time"`
	ReqDuration time.Duration `json:"req_duration"`
	URL         string        `json:"req_url"`
	RemoteAddr  string        `json:"remote_addr"`
	HTTPStatus  int           `json:"http_status"`
	Indexer     *IndexerInfo  `json:"indexer"`
	RPC         *RPCInfo      `json:"rpc"`
}

type MetricsCollector struct {
	Info   *MetricsInfo
	logger Logger
}

func (i *MetricsInfo) ToJSON() (string, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(i)
	if err == nil {
		return buf.String(), nil
	} else {
		return "", err
	}
}

func NewMetricsCollector(logger Logger) *MetricsCollector {
	return &MetricsCollector{
		Info: &MetricsInfo{
			Indexer: &IndexerInfo{},
			RPC:     &RPCInfo{},
		},
		logger: logger,
	}
}

func (m *MetricsCollector) Log() {
	if m.logger != nil {
		m.logger.Log(m.Info)
	}
}
