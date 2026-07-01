package history

import (
	"encoding/json"
	"testing"
)

func FuzzHistoryEntry(f *testing.F) {
	seeds := []string{
		`{"host":"example.com","daysLeft":100,"ts":"2026-01-01T00:00:00Z"}`,
		`{"host":"","daysLeft":0,"ts":""}`,
		`{}`,
		`{"host":"test.com","daysLeft":-1,"ts":"invalid-date"}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var e Entry
		_ = json.Unmarshal(data, &e)
	})
}
