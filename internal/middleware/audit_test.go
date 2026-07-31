package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMaskSensitiveFields_ArrayMultipleMatches 验证数组中多个同名敏感字段全部被脱敏
func TestMaskSensitiveFields_ArrayMultipleMatches(t *testing.T) {
	input := `[{"private_key":"aaa","name":"n1"},{"private_key":"bbb","name":"n2"}]`
	out := maskSensitiveFields(input)

	var arr []map[string]string
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("output is not valid JSON: %v, out=%s", err, out)
	}
	if len(arr) != 2 {
		t.Fatalf("expect 2 items, got %d", len(arr))
	}
	for i, item := range arr {
		if item["private_key"] != maskedPlaceholder {
			t.Errorf("item[%d].private_key not masked: %s", i, item["private_key"])
		}
		if item["name"] == "" || item["name"] == maskedPlaceholder {
			t.Errorf("item[%d].name should be preserved, got %q", i, item["name"])
		}
	}
}

// TestMaskSensitiveFields_EscapedChars 验证含转义字符的值不会破坏 JSON 结构
func TestMaskSensitiveFields_EscapedChars(t *testing.T) {
	input := `{"password":"p\"a\\ss","next":"visible"}`
	out := maskSensitiveFields(input)

	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v, out=%s", err, out)
	}
	if m["password"] != maskedPlaceholder {
		t.Errorf("password not masked: %s", m["password"])
	}
	if m["next"] != "visible" {
		t.Errorf("next field corrupted, got %q", m["next"])
	}
}

// TestMaskSensitiveFields_NestedAndCaseInsensitive 验证嵌套结构与大小写变体
func TestMaskSensitiveFields_NestedAndCaseInsensitive(t *testing.T) {
	input := `{"outer":{"PrivateKey":"x","apiKey":"y","normal":"z"}}`
	out := maskSensitiveFields(input)

	var m map[string]map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v, out=%s", err, out)
	}
	if m["outer"]["PrivateKey"] != maskedPlaceholder {
		t.Errorf("PrivateKey not masked: %s", m["outer"]["PrivateKey"])
	}
	if m["outer"]["apiKey"] != maskedPlaceholder {
		t.Errorf("apiKey not masked: %s", m["outer"]["apiKey"])
	}
	if m["outer"]["normal"] != "z" {
		t.Errorf("normal field corrupted: %s", m["outer"]["normal"])
	}
}

// TestMaskSensitiveFields_NonJSONFallback 验证非 JSON 字符串走 fallback 也能脱敏
func TestMaskSensitiveFields_NonJSONFallback(t *testing.T) {
	// 首字符非 { / [ 走 fallback 路径
	input := `some text "password":"secretVal" trailing "password":"another"`
	out := maskSensitiveFields(input)
	if strings.Contains(out, "secretVal") || strings.Contains(out, "another") {
		t.Errorf("fallback did not mask all occurrences: %s", out)
	}
	if strings.Count(out, maskedPlaceholder) < 2 {
		t.Errorf("expect at least 2 masked occurrences, got: %s", out)
	}
}

// TestBodyResponseWriter_Truncate 验证大响应截断到 maxBodyBytes，但下游写入完整
func TestBodyResponseWriter_Truncate(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &bodyResponseWriter{
		ResponseWriter: rec,
		statusCode:     200,
		body:           &strings.Builder{},
		maxBodyBytes:   16,
	}

	payload := strings.Repeat("A", 100)
	n, err := w.Write([]byte(payload))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	if n != len(payload) {
		t.Errorf("Write should return full length, got %d want %d", n, len(payload))
	}
	if w.body.Len() != 16 {
		t.Errorf("body buffer should be truncated to 16, got %d", w.body.Len())
	}
	if rec.Body.Len() != len(payload) {
		t.Errorf("downstream should receive full payload, got %d want %d", rec.Body.Len(), len(payload))
	}
}

// TestBodyResponseWriter_MultipleWrites 验证多次写入累计到上限后停止缓冲
func TestBodyResponseWriter_MultipleWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &bodyResponseWriter{
		ResponseWriter: rec,
		statusCode:     200,
		body:           &strings.Builder{},
		maxBodyBytes:   10,
	}

	_, _ = w.Write([]byte("12345"))
	_, _ = w.Write([]byte("67890"))
	_, _ = w.Write([]byte("ABCDE"))

	if w.body.Len() != 10 {
		t.Errorf("buffer len want 10, got %d, content=%q", w.body.Len(), w.body.String())
	}
	if w.body.String() != "1234567890" {
		t.Errorf("buffer content unexpected: %q", w.body.String())
	}
	if rec.Body.String() != "1234567890ABCDE" {
		t.Errorf("downstream body unexpected: %q", rec.Body.String())
	}
}
