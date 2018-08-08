package utils

import (
	"testing"
)

func TestGetCurrentTimeStamp(t *testing.T) {
	currentTime, err := GetCurrentTimeStamp([]string{})
	if currentTime == nil {
		t.Errorf("failed to get current time for empty timestamps array")
		return
	}

	timestamps := []string{"2015-01-01T00:00:00.000Z"}
	currentTime, err = GetCurrentTimeStamp(timestamps)
	if err != nil {
		t.Errorf("failed to parse current time: %v", timestamps)
		return
	}

	const ISOFormat = "2006-01-02T15:04:05.000Z"
	if timestamps[0] != currentTime.Format(ISOFormat) {
		t.Errorf("failed to get current time: %v", timestamps)
		return
	}
}
