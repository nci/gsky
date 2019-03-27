package main
import (
	"log"
	"net/http"

	"github.com/nci/gsky/utils"
)

func dapHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("dap url: %v", r.URL)

	dataFile := "/local/test_nd_tensor.tiff"
	//varName := "var1"
	verbose := true

	err := utils.WriteDap4(w, dataFile, verbose)
	if err != nil {
		log.Printf("DAP: error: %v", err)
	}
}
