package utils

import (
	"encoding/json"
	"testing"
)

func testUnmarshal(t *testing.T) {
	type TestJSON struct {
		Field1 string  `json:"field1"`
		Field2 int     `json:"field2"`
		Field3 float64 `json:"field3"`
	}

	jsonStr := []byte(`{ "field1": "test", "field2": 123, "field3": 12.3 }`)

	var testJSON1 TestJSON
	json.Unmarshal(jsonStr, &testJSON1)

	var testJSON2 TestJSON
	Unmarshal(jsonStr, &testJSON2)

	if testJSON1 != testJSON2 {
		t.Errorf("testJSON1 and testJSON2 are not equal: %v, %v", testJSON1, testJSON2)
	}

	jsonStr = []byte(`{ "field1": "test", "field2": 123, "field3: 12.3 }`)
	err := Unmarshal(jsonStr, &testJSON2)
	if err == nil {
		t.Errorf("invalid json does not return parsing error %v", jsonStr)
	}
}
