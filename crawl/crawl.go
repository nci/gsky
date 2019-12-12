package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	extr "github.com/nci/gsky/crawl/extractor"
	"github.com/nci/gsky/utils"
)

func ensure(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

const DefaultConcLimit = 2

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide a path to a file or '-' for reading from stdin")
	}

	path := os.Args[1]

	concLimit := DefaultConcLimit
	approx := true
	sentinel2Yaml := false
	landsatYaml := false
	var configFile string
	ncMetadata := false
	var outputFormat string

	if len(os.Args) > 2 {
		flagSet := flag.NewFlagSet("Usage", flag.ExitOnError)
		flagSet.IntVar(&concLimit, "conc", DefaultConcLimit, "Concurrent limit on processing subdatasets")
		var exact bool
		flagSet.BoolVar(&exact, "exact", false, "Compute exact statistics")
		flagSet.BoolVar(&sentinel2Yaml, "sentinel2_yaml", false, "Extract sentinel2 metadata from its yaml files")
		flagSet.StringVar(&configFile, "conf", "", "Crawl config file")
		flagSet.BoolVar(&ncMetadata, "nc_md", false, "Look for netCDF metadata")
		flagSet.BoolVar(&landsatYaml, "landsat_yaml", false, "Extract landsat metadata from its yaml files")
		flagSet.StringVar(&outputFormat, "fmt", "raw", "Output format. Valid values include raw and tsv")
		flagSet.Parse(os.Args[2:])

		approx = !exact
	}

	outputFormat = strings.ToLower(strings.TrimSpace(outputFormat))
	if len(outputFormat) == 0 {
		outputFormat = "raw"
	}

	if outputFormat != "raw" && outputFormat != "tsv" {
		log.Fatal("Valid output formats are raw and tsv")
	}

	var err error
	var pathList []string
	if path == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			file := scanner.Text()
			file = strings.TrimSpace(file)
			if len(file) == 0 {
				continue
			}
			pathList = append(pathList, file)
		}
		err = scanner.Err()
		ensure(err)
	}

	if len(pathList) == 0 {
		ensure(fmt.Errorf("No files from STDIN"))
	}

	var cfg []byte
	var iPath int
	for iPath, path = range pathList {
		var geoFile *extr.GeoFile
		if sentinel2Yaml {
			geoFile, err = extr.ExtractYaml(path, "sentinel2")
		} else if landsatYaml {
			geoFile, err = extr.ExtractYaml(path, "landsat")
		} else {
			config := &extr.Config{}
			if len(configFile) > 0 {
				if len(cfg) == 0 {
					cfg, err = ioutil.ReadFile(configFile)
					ensure(err)
					err = utils.Unmarshal([]byte(cfg), config)
					ensure(err)
				}
			} else {
				if ncMetadata {
					ruleSet := extr.RuleSet{
						NcMetadata:    ncMetadata,
						NameSpace:     extr.NSDataset,
						SRSText:       extr.SRSDetect,
						Proj4Text:     extr.Proj4Detect,
						Pattern:       `.+`,
						MatchFullPath: true,
						TimeAxis:      &extr.DatasetAxis{},
					}
					config.RuleSets = append(config.RuleSets, ruleSet)
				}
			}
			geoFile, err = extr.ExtractGDALInfo(path, concLimit, approx, config)
		}
		if err == nil {
			out, err := json.Marshal(&geoFile)
			ensure(err)

			outStr := string(out)
			if outputFormat == "tsv" {
				outStr = fmt.Sprintf("%s\tgdal\t%s", path, string(out))
			}

			var rec string
			if iPath < len(pathList)-1 {
				rec = fmt.Sprintf("%s\n", outStr)
			} else {
				rec = fmt.Sprintf("%s", outStr)
			}
			fmt.Print(rec)
		} else {
			os.Stderr.Write([]byte(err.Error()))
		}
	}
}
