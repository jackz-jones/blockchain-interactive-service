package sdk

import (
	"strings"
	"testing"
)

// TestParseUint_ValidZero 验证合法的 0 值不会被误判为无效
func TestParseUint_ValidZero(t *testing.T) {
	got, err := parseUint("0", 8)
	if err != nil {
		t.Fatalf("parseUint(\"0\", 8) should be valid, got err: %v", err)
	}
	if got != 0 {
		t.Fatalf("parseUint(\"0\", 8) expected 0, got %d", got)
	}
}

func TestParseUint_NegativeRejected(t *testing.T) {
	if _, err := parseUint("-1", 8); err == nil {
		t.Fatal("parseUint(\"-1\", 8) should return error")
	}
}

func TestParseUint_Overflow(t *testing.T) {
	// 256 超出 uint8 范围
	if _, err := parseUint("256", 8); err == nil {
		t.Fatal("parseUint(\"256\", 8) should overflow")
	}
}

func TestParseUint_Empty(t *testing.T) {
	if _, err := parseUint("", 32); err == nil {
		t.Fatal("parseUint(\"\", 32) should return error")
	}
}

func TestParseInt_ValidZero(t *testing.T) {
	got, err := parseInt("0", 16)
	if err != nil {
		t.Fatalf("parseInt(\"0\", 16) should be valid, got err: %v", err)
	}
	if got != 0 {
		t.Fatalf("parseInt(\"0\", 16) expected 0, got %d", got)
	}
}

func TestParseInt_Negative(t *testing.T) {
	got, err := parseInt("-128", 8)
	if err != nil {
		t.Fatalf("parseInt(\"-128\", 8) should be valid, got err: %v", err)
	}
	if got != -128 {
		t.Fatalf("expected -128, got %d", got)
	}
}

func TestParseInt_Overflow(t *testing.T) {
	// 128 超出 int8 范围
	if _, err := parseInt("128", 8); err == nil {
		t.Fatal("parseInt(\"128\", 8) should overflow")
	}
}

// TestConvertABIParam_Uint8Zero 验证 uint8=0 场景
func TestConvertABIParam_Uint8Zero(t *testing.T) {
	got, err := convertABIParam("uint8", []byte("0"))
	if err != nil {
		t.Fatalf("convertABIParam uint8 \"0\" failed: %v", err)
	}
	if v, ok := got.(uint8); !ok || v != 0 {
		t.Fatalf("expected uint8(0), got %v (%T)", got, got)
	}
}

// TestConvertABIParam_Bool 验证 bool 类型
func TestConvertABIParam_Bool(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
	}
	for _, c := range cases {
		got, err := convertABIParam("bool", []byte(c.in))
		if err != nil {
			t.Fatalf("bool(%q) err: %v", c.in, err)
		}
		if got.(bool) != c.want {
			t.Fatalf("bool(%q) expected %v, got %v", c.in, c.want, got)
		}
	}
}

// TestConvertABIParam_Bytes32_LengthMismatch 验证 bytes32 长度校验
func TestConvertABIParam_Bytes32_LengthMismatch(t *testing.T) {
	// 只有 16 字节（32 个 hex 字符）
	shortHex := "0x" + strings.Repeat("ab", 16)
	_, err := convertABIParam("bytes32", []byte(shortHex))
	if err == nil {
		t.Fatal("bytes32 with 16-byte payload should be rejected")
	}
}

// TestConvertABIParam_Bytes32_OK 验证 bytes32 合法值
func TestConvertABIParam_Bytes32_OK(t *testing.T) {
	// 32 字节 = 64 hex 字符
	hex := "0x" + strings.Repeat("cd", 32)
	got, err := convertABIParam("bytes32", []byte(hex))
	if err != nil {
		t.Fatalf("bytes32 valid should pass, got err: %v", err)
	}
	arr, ok := got.([32]byte)
	if !ok {
		t.Fatalf("expected [32]byte, got %T", got)
	}
	if arr[0] != 0xcd || arr[31] != 0xcd {
		t.Fatalf("byte content mismatch: %x", arr)
	}
}

// TestConvertABIParam_Uint256 验证大整数
func TestConvertABIParam_Uint256(t *testing.T) {
	got, err := convertABIParam("uint256", []byte("1000000000000000000"))
	if err != nil {
		t.Fatalf("uint256 err: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil big.Int")
	}
}
