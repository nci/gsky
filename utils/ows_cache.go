package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type OWSCache struct {
	MASAddress string
	GPath      string
	verbose    bool
}

func NewOWSCache(masAddress, gpath string, verbose bool) *OWSCache {
	return &OWSCache{
		MASAddress: masAddress,
		GPath:      gpath,
		verbose:    verbose,
	}
}

func (o *OWSCache) Put(query string, value string) error {
	reqURL := fmt.Sprintf("http://%s%s?put_ows_cache&query=%s", o.MASAddress, o.GPath, query)
	postBody := url.Values{"value": {value}}
	if o.verbose {
		log.Printf("querying MAS for OWSCache Put: %v", reqURL)
	}
	resp, err := http.PostForm(reqURL, postBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type PutStatus struct {
		Error string `json:"error"`
	}

	var status PutStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		return err
	}

	if len(status.Error) > 0 {
		return fmt.Errorf("%s", status.Error)
	}
	return nil
}

func (o *OWSCache) Get(query string) ([]byte, error) {
	var result []byte
	url := fmt.Sprintf("http://%s%s?get_ows_cache&query=%s", o.MASAddress, o.GPath, query)
	if o.verbose {
		log.Printf("querying MAS for OWSCache Get: %v", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	result, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (o *OWSCache) GetConfig(query string) (*Config, error) {
	value, err := o.Get(query)
	if err != nil {
		return nil, err
	}
	type cacheResult struct {
		Error  string  `json:"error"`
		Config *Config `json:"value"`
	}

	var result cacheResult
	err = json.Unmarshal(value, &result)
	if err != nil {
		return nil, err
	}

	if len(result.Error) > 0 {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return result.Config, nil
}

func FindConfigGPath(config *Config) string {
	if len(strings.TrimSpace(config.ServiceConfig.OWSCacheGPath)) > 0 {
		return config.ServiceConfig.OWSCacheGPath
	}

	if len(config.Layers) == 0 {
		return ""
	}
	var layerList []*Layer
	for iLayer := range config.Layers {
		layer := &config.Layers[iLayer]
		if hasBlendedService(layer) {
			continue
		}
		if len(strings.TrimSpace(layer.DataSource)) == 0 {
			continue
		}
		layerList = append(layerList, layer)
	}

	sort.Slice(layerList, func(i, j int) bool { return layerList[i].Name <= layerList[j].Name })
	return layerList[0].DataSource
}
