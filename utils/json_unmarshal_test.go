package utils

import (
	"encoding/json"
	"testing"
)

func testUnmarshal(t *testing.T) {
	type TestJson struct {
		Field1 string  `json:"field1"`
		Field2 int     `json:"field2"`
		Field3 float64 `json:"field3"`
	}

	jsonStr := []byte(`{ "field1": "test", "field2": 123, "field3": 12.3 }`)

	var testJson1 TestJson
	json.Unmarshal(jsonStr, &testJson1)

	var testJson2 TestJson
	Unmarshal(jsonStr, &testJson2)

	if testJson1 != testJson2 {
		t.Errorf("testJson1 and testJson2 are not equal: %v, %v", testJson1, testJson2)
	}

	jsonStr = []byte(`{ "field1": "test", "field2": 123, "field3: 12.3 }`)
	err := Unmarshal(jsonStr, &testJson2)
	if err == nil {
		t.Errorf("invalid json does not return parsing error %v", jsonStr)
	}
}
