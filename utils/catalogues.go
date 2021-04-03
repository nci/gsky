package utils

import (
	"strings"
	"path/filepath"
	"os"
	"io"
	"encoding/json"
//	"bytes"
	"log"
)

/*
import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/edisonguo/jet"
)
*/

type CatalogueHandler struct {
	Path string
	URLRoot string
	StaticRoot string
	MasAddress string
	IndexTemplateRoot string
	Output io.Writer
}

func NewCatalogueHandler(path string, urlRoot, staticRoot string, masAddress string, indexTemplateRoot string, output io.Writer) *CatalogueHandler {
	return & CatalogueHandler {
		Path: path,
		URLRoot: urlRoot,
		StaticRoot: staticRoot,
		MasAddress: masAddress,
		IndexTemplateRoot: indexTemplateRoot,
		Output: output,
	}
}

func (h *CatalogueHandler) Process() {
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

	log.Printf("1111 %v, %v", absPath, h.StaticRoot)
	if !strings.HasPrefix(absPath, h.StaticRoot) {
		return
	}

	if _, err := os.Stat(absPath); err == nil {
		log.Printf("found index file: %v", absPath)
	} else {
		indexPath := h.Path
		if len(ext) > 0 {
			indexPath = filepath.Dir(indexPath)
		}
		if len(indexPath) == 0 {
			log.Printf("root catalogue")
		} else {
			log.Printf("non root catalogue")
			h.renderCataloguePage(indexPath)
		}
	}
}

type Anchor struct {
	URL string
	Title string
}

type RenderData struct {
	Navigations []Anchor
	Endpoints []Anchor
	Title string
}

func (h *CatalogueHandler) renderCataloguePage(indexPath string) {
	/*
	url := strings.Replace(fmt.Sprintf("http://%s%s?timestamps", g.MasAddress, indexPath), " ", "%20", -1)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("MAS http error: %v,%v", url, err)
		return emptyDates, token
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("MAS http error: %v,%v", url, err)
		return emptyDates, token
	}
	*/

	body := []byte(`{"sub_paths": ["/g/data/xu18/ga/ga_ls7e_ard_3/088/080/1999", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2000", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2001", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2002", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2003", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2004", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2005", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2006", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2007", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2008", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2009", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2010", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2011", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2012", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2013", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2014", "/g/data/xu18/ga/ga_ls7e_ard_3/088/080/2015"], "has_namespaces": false}`)

	type GPathInfo struct {
		Error      string   `json:"error"`
		Paths []string `json:"sub_paths"`
		HasNamespaces bool `json:"has_namespaces"`
	}

	var gpathInfo GPathInfo
	err := json.Unmarshal(body, &gpathInfo)
	if err != nil {
		log.Printf("MAS json response error: %v", err)
		return
	}

	var renderData RenderData	

	parts := strings.Split(indexPath, "/")
	if len(parts) > 0 {
		renderData.Title = parts[len(parts)-1]
	}

	var homeRelParts []string
	for ii := 0; ii < len(parts); ii++ {
		homeRelParts = append(homeRelParts, "..")
	}
	relHomePath := strings.Join(homeRelParts, "/")
	renderData.Navigations = append(renderData.Navigations, Anchor {URL: relHomePath, Title: "Home"})
	for ip, p := range parts[:len(parts)-1] {
		var relParts []string
		for ii := len(parts)-1; ii >= ip; ii-- {
			relParts = append(relParts, "..")
		} 
		relPath := strings.Join(relParts, "/")
		a := Anchor {
			URL: filepath.Join(h.URLRoot, relPath),
			Title: p,
		}
		renderData.Navigations = append(renderData.Navigations, a)
	}

	for _, path := range gpathInfo.Paths {
		subPath := filepath.Base(path)
		a := Anchor {
			URL: "get caps url",
			Title: subPath,
		}
		renderData.Endpoints = append(renderData.Endpoints, a)
	}

	err = ExecuteWriteTemplateFile(h.Output, renderData, filepath.Join(h.IndexTemplateRoot, "catalogue_index.tpl"))
	if err != nil {
		log.Printf("%v", err)
		return
	}
}
