package errcode

import (
	"errors"
	"net/http"
	"testing"
)

func TestBizErrorBasic(t *testing.T) {
	e := New(InvalidParam, "bad input")
	if e.Code != InvalidParam {
		t.Fatalf("code mismatch: %d", e.Code)
	}
	if e.Error() == "" {
		t.Fatal("Error() should not be empty")
	}
	if HTTPStatusOf(e) != http.StatusBadRequest {
		t.Fatalf("http status mismatch: %d", HTTPStatusOf(e))
	}
}

func TestBizErrorWrapAndUnwrap(t *testing.T) {
	root := errors.New("boom")
	e := Wrap(InternalError, "internal", root)
	if !errors.Is(e, root) {
		t.Fatal("errors.Is should walk to root cause")
	}
	be, ok := AsBizError(e)
	if !ok || be.Code != InternalError {
		t.Fatal("AsBizError should extract BizError")
	}
	if HTTPStatusOf(e) != http.StatusInternalServerError {
		t.Fatalf("status mismatch: %d", HTTPStatusOf(e))
	}
}

func TestHTTPStatusOfMappings(t *testing.T) {
	tests := []struct {
		code Code
		want int
	}{
		{OK, http.StatusOK},
		{InvalidParam, http.StatusBadRequest},
		{NotFound, http.StatusNotFound},
		{Conflict, http.StatusConflict},
		{TooManyRequests, http.StatusTooManyRequests},
		{Unauthorized, http.StatusUnauthorized},
		{Forbidden, http.StatusForbidden},
		{QuotaExceededDaily, http.StatusPaymentRequired},
		{SDKCallFailed, http.StatusBadGateway},
		{InternalError, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		if got := defaultHTTPStatus(tc.code); got != tc.want {
			t.Errorf("code=%d want=%d got=%d", tc.code, tc.want, got)
		}
	}
}

func TestExplicitHTTPStatusWins(t *testing.T) {
	e := &BizError{Code: InvalidParam, HTTPStatus: http.StatusTeapot}
	if HTTPStatusOf(e) != http.StatusTeapot {
		t.Fatalf("explicit http status should win, got %d", HTTPStatusOf(e))
	}
}
