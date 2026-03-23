package util

import (
	"testing"
)

func TestConvertToLogFields(t *testing.T) {
	m := map[string]interface{}{
		"a": 123,
	}

	logFields := ConvertToLogFields(m)
	if len(logFields) != 1 {
		t.Errorf("wanted length 3,got length %d", len(logFields))
	}

	if logFields[0].Key != "a" && logFields[0].Value != 123 {
		t.Errorf("wanted kv a:123,got kv %s:%d", logFields[0].Key, logFields[0].Value)
	}
}
