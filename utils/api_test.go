package utils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGenerateDatesMas(t *testing.T) {
	const ISOFormat = "2006-01-02T15:04:05.000Z"

	masAddress := "127.0.0.1:8888"
	collection := "/g/data2/tc43/modis-fc/v310/tiles/monthly/cover"
	namespaces := []string{"bare_soil", "phot_veg", "nphot_veg"}

	step, _ := time.ParseDuration("0s")
	res := GenerateDatesMas("2001-01-02", "2015-01-01T00:00:00.000Z", masAddress, collection, namespaces, step)
	if len(res) != 0 {
		t.Errorf("Start date test failed. Expecting empty output, actual: %v", res)
		return
	}

	res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2015-01-01T00:00:00", masAddress, collection, namespaces, step)
	if len(res) != 0 {
		t.Errorf("End date test failed. Expecting empty output, actual: %v", res)
		return
	}

	res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2015-01-01T00:00:00.000Z", "127.0.0.0", collection, namespaces, step)
	if len(res) != 0 {
		t.Errorf("MAS connection test failed. Expecting empty output, actual: %v", res)
		return
	}

	testURL := strings.Replace(fmt.Sprintf("http://%s%s?timestamps&time=%s&since=%s&namespace=%s", masAddress, collection, "", "", namespaces), " ", "%20", -1)
	_, err := http.Get(testURL)
	masOnline := err == nil

	if masOnline {
		res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2015-01-01T00:00:00.000Z", masAddress, "no_collection", namespaces, step)
		if len(res) != 0 {
			t.Errorf("Collection test failed. Expecting empty output, actual: %v", res)
			return
		}

		res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2015-01-01T00:00:00.000Z", masAddress, collection, []string{"no_namespace"}, step)
		if len(res) != 0 {
			t.Errorf("Namespace test failed. Expecting empty output, actual: %v", res)
			return
		}

		res = GenerateDatesMas("", "2015-01-01T00:00:00.000Z", masAddress, collection, namespaces, step)
		if len(res) == 0 {
			t.Errorf("Empty start date test failed. Expecting some outputs, but got empty ouputs")
			return
		}

		res = GenerateDatesMas("   ", "2015-01-01T00:00:00.000Z", masAddress, collection, namespaces, step)
		if len(res) == 0 {
			t.Errorf("Empty start date test failed. Expecting some outputs, but got empty ouputs")
			return
		}

		res = GenerateDatesMas("", "", masAddress, collection, namespaces, step)
		if len(res) == 0 {
			t.Errorf("Empty end date test failed. Expecting some outputs, but got empty ouputs")
			return
		}

		res = GenerateDatesMas("", "   ", masAddress, collection, namespaces, step)
		if len(res) == 0 {
			t.Errorf("Empty end date test failed. Expecting some outputs, but got empty ouputs")
			return
		}

		res = GenerateDatesMas("", "", masAddress, collection, []string{}, step)
		if len(res) == 0 {
			t.Errorf("Empty namespace test failed. Expecting some outputs, but got empty ouputs")
			return
		}

		for _, ts := range res {
			_, err := time.Parse(ISOFormat, ts)
			if err != nil {
				t.Errorf("Timestamp test failed. The timestamps returned from server are not in ISO format: %v", ts)
				return
			}
		}

		step, _ = time.ParseDuration(fmt.Sprintf("%dh", 24*60))
		res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2018-01-01T00:00:00.000Z", masAddress, collection, namespaces, step)
		if len(res) < 2 {
			t.Errorf("number of timestamps < 2: %v", res)
			return
		}

		t0, _ := time.Parse(ISOFormat, res[0])
		t1, _ := time.Parse(ISOFormat, res[1])
		if t1.Sub(t0) != step {
			t.Errorf("timestamps aggregation failed: %v", res)
			return
		}

	} else {
		t.Skip("MAS endpoint is unavailable. Skipping tests that require MAS connection")
	}

}

func TestGetLayerDates(t *testing.T) {
	config := &Config{}

	config.Layers = append(config.Layers, Layer{StartISODate: "", EndISODate: "", TimeGen: "yearly"})
	config.GetLayerDates(0)
	if len(config.Layers[0].Dates) > 0 {
		t.Errorf("Invalid date string but got successfully converted: %v\n", config.Layers[0].Dates)
		return
	}

	config.Layers[0] = Layer{StartISODate: "2015-01-01T00:00:00.000Z", EndISODate: "", TimeGen: "yearly"}
	config.GetLayerDates(0)
	if len(config.Layers[0].Dates) > 0 {
		t.Errorf("Invalid date string but got successfully converted: %v\n", config.Layers[0].Dates)
		return
	}

	config.Layers[0] = Layer{StartISODate: "2015-01-01T00:00:00.000Z", EndISODate: "2018-01-01T00:00:00.000Z", TimeGen: "yearly"}
	config.GetLayerDates(0)
	if len(config.Layers[0].Dates) != 3 {
		t.Errorf("Failed to generate dates: %v\n", config.Layers[0].Dates)
		return
	}

	config.Layers[0] = Layer{StartISODate: "2015-01-01T00:00:00.000Z", EndISODate: "now", TimeGen: "yearly"}
	config.GetLayerDates(0)
	if len(config.Layers[0].Dates) == 0 {
		t.Errorf("Failed to parse now() as end date: %#v\n", config.Layers[0])
		return
	}
}

func TestLoadConfigFileTemplate(t *testing.T) {
	testInclude := `
some text with newline
{{ .["title"] }}
double quotes" more quotes""
back slashes \\more slashes
text with tab	te	xt

`

	testConfig := `
{
  "layers":
  [
    {
	  "name": $gdoc${{include "test_include" map("title", "some name") }}$gdoc$,
	  "title": 
	$gdoc$

	{{include "/tmp/test_include" map("title", "some title") }}

	$gdoc$
    }
  ]
}
`

	expectedName := strings.Replace(testInclude, `{{ .["title"] }}`, "some name", -1)
	expectedTitle := strings.Replace(testInclude, `{{ .["title"] }}`, "some title", -1)

	err := ioutil.WriteFile("/tmp/test_config", []byte(testConfig), 0644)
	if err != nil {
		t.Errorf("Test setup failed: %v", err)
		return
	}
	defer os.Remove("/tmp/test_config")

	err = ioutil.WriteFile("/tmp/test_include", []byte(testInclude), 0644)
	if err != nil {
		t.Errorf("Test setup failed: %v", err)
		return
	}
	defer os.Remove("/tmp/test_include")

	config := &Config{}
	err = config.LoadConfigFile("/tmp/gsky_config_not_found", false)
	if err == nil {
		t.Errorf("failed to parse config file")
		return
	}

	err = config.LoadConfigFile("/tmp/test_config", false)
	if err != nil {
		t.Errorf("Failed to load config: %v", err)
		return
	}

	if len(config.Layers) != 1 {
		t.Errorf("Failed to parse layers: %v", config.Layers)
		return
	}

	if config.Layers[0].Name != expectedName {
		t.Errorf("Failed to parse included template for name: %v, expected: %v", config.Layers[0].Name, expectedName)
		return
	}

	title := strings.TrimSpace(config.Layers[0].Title)
	trimmedExpected := strings.TrimSpace(expectedTitle)
	if title != trimmedExpected {
		t.Errorf("Failed to parse included template for title: %v, expected: %v", title, trimmedExpected)
		return
	}

}

func TestGenerateDatesMCD43A4(t *testing.T) {
	const ISOFormat = "2006-01-02T15:04:05.000Z"

	start, _ := time.Parse(ISOFormat, "2001-01-02T00:00:00.000Z")
	end, _ := time.Parse(ISOFormat, "2002-01-02T00:00:00.000Z")
	step, _ := time.ParseDuration("0s")
	timestamps := GenerateDatesMCD43A4(start, end, step)
	if len(timestamps) > 0 {
		t.Errorf("Failed to handle zero time step")
		return
	}

	step, _ = time.ParseDuration("72h")
	timestamps = GenerateDatesMCD43A4(start, end, step)
	if len(timestamps) == 0 {
		t.Errorf("Failed to handle non-zero time step, %v, %v, %v", start, end, step)
		return
	}
}

func TestGenerateDates(t *testing.T) {
	const ISOFormat = "2006-01-02T15:04:05.000Z"

	start, _ := time.Parse(ISOFormat, "2001-01-02T00:00:00.000Z")
	end, _ := time.Parse(ISOFormat, "2002-01-02T00:00:00.000Z")
	step, _ := time.ParseDuration("0s")

	timestamps := GenerateDates("not_found", start, end, step)
	if len(timestamps) > 0 {
		t.Errorf("Non-existing timestamp generator successfully generates timestamps")
		return
	}

}
