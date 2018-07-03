package utils

import (
	"testing"
	"time"
)

func TestGenerateDatesMas(t *testing.T) {
	const ISOFormat = "2006-01-02T15:04:05.000Z"

	masAddress := "127.0.0.1:8888"
	collection := "/g/data2/tc43/modis-fc/v310/tiles/monthly/cover"
	namespaces := []string{"bare_soil", "phot_veg", "nphot_veg"}

	res := GenerateDatesMas("2001-01-02", "2015-01-01T00:00:00.000Z", masAddress, collection, namespaces)
	if len(res) != 0 {
		t.Errorf("Start date test failed. Expecting empty output, actual: %v", res)
	}

	res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2015-01-01T00:00:00", masAddress, collection, namespaces)
	if len(res) != 0 {
		t.Errorf("End date test failed. Expecting empty output, actual: %v", res)
	}

	res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2015-01-01T00:00:00.000Z", "127.0.0.0", collection, namespaces)
	if len(res) != 0 {
		t.Errorf("MAS connection test failed. Expecting empty output, actual: %v", res)
	}

	res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2015-01-01T00:00:00.000Z", masAddress, "no_collection", namespaces)
	if len(res) != 0 {
		t.Errorf("Collection test failed. Expecting empty output, actual: %v", res)
	}

	res = GenerateDatesMas("2015-01-02T00:00:00.000Z", "2015-01-01T00:00:00.000Z", masAddress, collection, []string{"no_namespace"})
	if len(res) != 0 {
		t.Errorf("Namespace test failed. Expecting empty output, actual: %v", res)
	}

	res = GenerateDatesMas("", "2015-01-01T00:00:00.000Z", masAddress, collection, namespaces)
	if len(res) == 0 {
		t.Errorf("Empty start date test failed. Expecting some outputs, but got empty ouputs")
	}

	res = GenerateDatesMas("   ", "2015-01-01T00:00:00.000Z", masAddress, collection, namespaces)
	if len(res) == 0 {
		t.Errorf("Empty start date test failed. Expecting some outputs, but got empty ouputs")
	}

	res = GenerateDatesMas("", "", masAddress, collection, namespaces)
	if len(res) == 0 {
		t.Errorf("Empty end date test failed. Expecting some outputs, but got empty ouputs")
	}

	res = GenerateDatesMas("", "   ", masAddress, collection, namespaces)
	if len(res) == 0 {
		t.Errorf("Empty end date test failed. Expecting some outputs, but got empty ouputs")
	}

	for _, ts := range res {
		_, err := time.Parse(ISOFormat, ts)
		if err != nil {
			t.Errorf("Timestamp test failed. The timestamps returned from server are not in ISO format: %v", ts)
		}
	}

}
