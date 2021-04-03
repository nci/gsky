package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const catalogueHomeTitle = "Home"

type CatalogueHandler struct {
	Path              string
	URLHost           string
	URLPathRoot       string
	StaticRoot        string
	MasAddress        string
	IndexTemplateRoot string
	Output            http.ResponseWriter
}

func NewCatalogueHandler(path, urlHost, urlPathRoot, staticRoot, masAddress, indexTemplateRoot string, output http.ResponseWriter) *CatalogueHandler {
	return &CatalogueHandler{
		Path:              path,
		URLHost:           urlHost,
		URLPathRoot:       urlPathRoot,
		StaticRoot:        staticRoot,
		MasAddress:        masAddress,
		IndexTemplateRoot: indexTemplateRoot,
		Output:            output,
	}
}

func (h *CatalogueHandler) Process() {
	h.Path = "/" + strings.Trim(h.Path, "/")
	ext := filepath.Ext(h.Path)

	indexFile := h.Path
	if len(ext) > 0 {
		if !strings.HasSuffix(h.Path, "index.html") {
			return
		} else {
			indexFile = filepath.Join(h.StaticRoot, indexFile)
		}
	} else {
		indexFile = filepath.Join(h.StaticRoot, indexFile, "index.html")
	}

	absPath, err := filepath.Abs(indexFile)
	if err != nil {
		return
	}

	if !strings.HasPrefix(absPath, h.StaticRoot) {
		return
	}

	if _, err := os.Stat(absPath); err == nil {
		type Payload struct {
			Host string
		}
		payload := &Payload{Host: h.URLHost}
		err := ExecuteWriteTemplateFile(h.Output, payload, absPath)
		if err != nil {
			http.Error(h.Output, err.Error(), 500)
			return
		}

	} else {
		indexPath := h.Path
		if len(ext) > 0 {
			indexPath = filepath.Dir(indexPath)
		}
		if indexPath == "/" || len(indexPath) == 0 {
			h.renderRootCataloguePage()
		} else {
			h.renderCataloguePage(indexPath)
		}
	}
}

type gpathMetadata struct {
	Error         string   `json:"error"`
	Paths         []string `json:"sub_paths"`
	HasNamespaces bool     `json:"has_namespaces"`
	PathRoot      string   `json:"gpath_root"`
}

type anchor struct {
	URL   string
	Title string
}

type renderData struct {
	Navigations []*anchor
	Endpoints   []*anchor
	Title       string
}

func (h *CatalogueHandler) renderRootCataloguePage() {
	gpathInfo, err := h.getGPathMetadata("", "list_root_gpath")
	if err != nil {
		log.Printf("%v", err)
		return
	}

	var rd renderData
	rd.Title = catalogueHomeTitle

	for _, path := range gpathInfo.Paths {
		urlPath := filepath.Join(h.URLPathRoot, path)
		a := &anchor{
			URL:   fmt.Sprintf("%s/%s", h.URLHost, urlPath),
			Title: path,
		}
		rd.Endpoints = append(rd.Endpoints, a)
	}

	err = ExecuteWriteTemplateFile(h.Output, rd, filepath.Join(h.IndexTemplateRoot, "catalogue_index.tpl"))
	if err != nil {
		log.Printf("%v", err)
		return
	}
}

func (h *CatalogueHandler) renderCataloguePage(indexPath string) {
	gpathInfo, err := h.getGPathMetadata(indexPath, "list_sub_gpath")
	if err != nil {
		log.Printf("%v", err)
		return
	}

	var rd renderData
	relIndexPath := indexPath[len(gpathInfo.PathRoot):]
	parts := strings.Split(relIndexPath, "/")
	if strings.HasPrefix(relIndexPath, "/") {
		parts = parts[1:]
	}

	urlRoot := fmt.Sprintf("%s/%s", h.URLHost, h.URLPathRoot)
	rd.Navigations = append(rd.Navigations, &anchor{URL: urlRoot, Title: catalogueHomeTitle})
	if len(parts) > 0 {
		if len(gpathInfo.PathRoot) < len(indexPath) {
			urlPath := fmt.Sprintf("%s/%s", h.URLHost, filepath.Join(h.URLPathRoot, gpathInfo.PathRoot))
			rd.Navigations = append(rd.Navigations, &anchor{URL: urlPath, Title: gpathInfo.PathRoot})
			rd.Title = parts[len(parts)-1]
		} else {
			rd.Title = gpathInfo.PathRoot
		}
	}

	for ip, p := range parts[:len(parts)-1] {
		relPath := filepath.Join(gpathInfo.PathRoot, strings.Join(parts[:ip+1], "/"))
		relPath = strings.Trim(relPath, "/")
		a := &anchor{
			URL:   fmt.Sprintf("%s/%s", urlRoot, relPath),
			Title: p,
		}
		rd.Navigations = append(rd.Navigations, a)
	}

	if gpathInfo.HasNamespaces {
		urlPath := filepath.Join("ows", indexPath)
		a := &anchor{
			URL:   fmt.Sprintf("%s/%s?service=WMS&request=GetCapabilities&version=1.3.0", h.URLHost, urlPath),
			Title: "WMS GetCapabilities",
		}
		rd.Endpoints = append(rd.Endpoints, a)
	}

	for _, path := range gpathInfo.Paths {
		subPath := filepath.Base(path)
		urlPath := filepath.Join(h.URLPathRoot, indexPath, subPath)
		a := &anchor{
			URL:   fmt.Sprintf("%s/%s", h.URLHost, urlPath),
			Title: subPath,
		}
		rd.Endpoints = append(rd.Endpoints, a)
	}

	err = ExecuteWriteTemplateFile(h.Output, rd, filepath.Join(h.IndexTemplateRoot, "catalogue_index.tpl"))
	if err != nil {
		log.Printf("%v", err)
		return
	}
}

func (h *CatalogueHandler) getGPathMetadata(indexPath string, queryOp string) (*gpathMetadata, error) {
	url := strings.Replace(fmt.Sprintf("http://%s%s?%s", h.MasAddress, indexPath, queryOp), " ", "%20", -1)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("MAS (%s) error: %v,%v", queryOp, url, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("MAS (%s) error: %v,%v", queryOp, url, err)
	}

	var gpathInfo gpathMetadata
	err = json.Unmarshal(body, &gpathInfo)
	if err != nil {
		return nil, fmt.Errorf("MAS (%s) json response error: %v", queryOp, err)
	}
	return &gpathInfo, nil
}
