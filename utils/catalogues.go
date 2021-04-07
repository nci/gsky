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

const CatalogueDirName = "catalogues"
const catalogueHomeTitle = "Home"

type CatalogueHandler struct {
	Path              string
	URLHost           string
	URLPathRoot       string
	StaticRoot        string
	MasAddress        string
	IndexTemplateRoot string
	Verbose           bool
	Output            http.ResponseWriter
}

func NewCatalogueHandler(path, urlHost, urlPathRoot, staticRoot, masAddress, indexTemplateRoot string, verbose bool, output http.ResponseWriter) *CatalogueHandler {
	return &CatalogueHandler{
		Path:              path,
		URLHost:           urlHost,
		URLPathRoot:       urlPathRoot,
		StaticRoot:        staticRoot,
		MasAddress:        masAddress,
		IndexTemplateRoot: indexTemplateRoot,
		Verbose:           verbose,
		Output:            output,
	}
}

const catalogueIndexFile = "index.html"
const catalogueGSKYLayerFile = "gsky_layers.json"
const catalogueTerriaCatalogFile = "terria_catalog.json"

func (h *CatalogueHandler) Process() int {
	if len(CheckIndexFile(h.StaticRoot, h.Path, h.Verbose)) > 0 {
		return 1
	}

	h.Output.Header().Set("Access-Control-Allow-Origin", "*")
	h.Output.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")

	indexPath := h.Path
	ext := filepath.Ext(indexPath)
	if len(ext) > 0 {
		if strings.HasSuffix(indexPath, catalogueGSKYLayerFile) {
			h.renderGSKYLayerFile(indexPath)
			return 0
		} else if strings.HasSuffix(indexPath, catalogueTerriaCatalogFile) {
			h.renderTerriaCatalogFile(indexPath)
			return 0
		} else {
			indexPath = filepath.Dir(indexPath)
		}
	}

	if indexPath == "/" || len(indexPath) == 0 {
		h.renderRootCataloguePage()
	} else {
		h.renderCataloguePage(indexPath)
	}

	return 0
}

func CheckIndexFile(staticRoot string, path string, verbose bool) string {
	path = "/" + strings.Trim(path, "/")
	ext := filepath.Ext(path)

	indexFile := path
	if len(ext) > 0 {
		isIndex := false
		for _, f := range []string{catalogueIndexFile, catalogueGSKYLayerFile, catalogueTerriaCatalogFile} {
			if strings.HasSuffix(path, f) {
				isIndex = true
				break
			}
		}
		if !isIndex {
			return ""
		} else {
			indexFile = filepath.Join(staticRoot, indexFile)
		}

	} else {
		indexFile = filepath.Join(staticRoot, indexFile, "index.html")
	}

	absPath, err := filepath.Abs(indexFile)
	if err != nil {
		if verbose {
			log.Printf("IsIndexFile: %v", err)
		}
		return ""
	}

	if !strings.HasPrefix(absPath, staticRoot) {
		if verbose {
			log.Printf("IsIndexFile absPath prefix: %v -> %v", path, absPath)
		}
		return ""
	}

	if _, err := os.Stat(absPath); err == nil {
		return absPath
	}
	return ""
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

func (h *CatalogueHandler) renderGSKYLayerFile(indexPath string) {
	namespace := filepath.Dir(indexPath)
	masLayers, err := LoadLayersFromMAS(h.MasAddress, namespace, h.Verbose)
	if err != nil {
		log.Printf("renderGSKYLayerFile: %v", err)
		return
	}

	for i := range masLayers.Layers {
		jsonEscapeLayer(&masLayers.Layers[i])
	}

	err = ExecuteWriteTemplateFile(h.Output, masLayers, filepath.Join(h.IndexTemplateRoot, "gsky_layers.tpl"))
	if err != nil {
		log.Printf("%v", err)
		return
	}
}

func (h *CatalogueHandler) renderTerriaCatalogFile(indexPath string) {
	namespace := filepath.Dir(indexPath)
	masLayers, err := LoadLayersFromMAS(h.MasAddress, namespace, h.Verbose)
	if err != nil {
		log.Printf("renderTerriaCatalogFile: %v", err)
		return
	}

	type RenderTerriaCatalog struct {
		Namespace string
		Layers    []Layer
	}

	terriaCatalog := &RenderTerriaCatalog{Namespace: jsonEscape(namespace)}
	terriaCatalog.Layers = make([]Layer, len(masLayers.Layers))
	for i := range masLayers.Layers {
		terriaCatalog.Layers[i] = masLayers.Layers[i]
		terriaCatalog.Layers[i].DataURL = jsonEscape(fmt.Sprintf("%s/%s", h.URLHost, filepath.Join("ows", namespace)))
		jsonEscapeLayer(&terriaCatalog.Layers[i])
	}

	err = ExecuteWriteTemplateFile(h.Output, terriaCatalog, filepath.Join(h.IndexTemplateRoot, "terria_catalog.tpl"))
	if err != nil {
		log.Printf("%v", err)
		return
	}
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
		wms := &anchor{
			URL:   fmt.Sprintf("%s/%s?service=WMS&request=GetCapabilities&version=1.3.0", h.URLHost, urlPath),
			Title: "WMS GetCapabilities",
		}
		rd.Endpoints = append(rd.Endpoints, wms)

		urlPath = filepath.Join(CatalogueDirName, indexPath, catalogueTerriaCatalogFile)
		terriaCatalog := &anchor{
			URL:   fmt.Sprintf("%s/%s", h.URLHost, urlPath),
			Title: "Terria Catalog",
		}
		rd.Endpoints = append(rd.Endpoints, terriaCatalog)

		urlPath = filepath.Join(CatalogueDirName, indexPath, catalogueGSKYLayerFile)
		gskyLayer := &anchor{
			URL:   fmt.Sprintf("%s/%s", h.URLHost, urlPath),
			Title: "GSKY Layers",
		}
		rd.Endpoints = append(rd.Endpoints, gskyLayer)
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
	if h.Verbose {
		log.Printf("catalogueHandler: %v", url)
	}
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
	if len(gpathInfo.Error) > 0 {
		return nil, fmt.Errorf("MAS (%s) json response error: %v", queryOp, gpathInfo.Error)
	}

	return &gpathInfo, nil
}

func jsonEscapeLayer(layer *Layer) {
	layer.Title = jsonEscape(layer.Title)
	layer.Name = jsonEscape(layer.Name)
	layer.DataSource = jsonEscape(layer.DataSource)
	layer.DataURL = jsonEscape(layer.DataURL)

	for i := range layer.RGBProducts {
		layer.RGBProducts[i] = jsonEscape(layer.RGBProducts[i])
	}

	for i := range layer.AxesInfo {
		layer.AxesInfo[i].Name = jsonEscape(layer.AxesInfo[i].Name)
		layer.AxesInfo[i].Default = jsonEscape(layer.AxesInfo[i].Default)
		for j := range layer.AxesInfo[i].Values {
			layer.AxesInfo[i].Values[j] = jsonEscape(layer.AxesInfo[i].Values[j])
		}
	}
}

func jsonEscape(i string) string {
	b, err := json.Marshal(i)
	if err != nil {
		log.Printf("Failed to JSON escape: %v", err)
		return i
	}
	s := string(b)
	return s[1 : len(s)-1]
}
