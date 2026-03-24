package pastehub

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPutGetDeleteBuffer(t *testing.T) {
	srv := NewServer(NewStore(), 1024)
	h := srv.Handler()

	putReq := httptest.NewRequest(http.MethodPut, "/v1/buffers/default", bytes.NewReader([]byte("hello")))
	putReq.RemoteAddr = "127.0.0.1:12345"
	putReq.Header.Set("Content-Type", "text/plain")
	putRes := httptest.NewRecorder()
	h.ServeHTTP(putRes, putReq)
	if putRes.Code != http.StatusCreated {
		t.Fatalf("PUT status = %d, want %d", putRes.Code, http.StatusCreated)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/buffers/default", nil)
	getReq.RemoteAddr = "127.0.0.1:12345"
	getRes := httptest.NewRecorder()
	h.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getRes.Code, http.StatusOK)
	}
	if body := getRes.Body.String(); body != "hello" {
		t.Fatalf("GET body = %q, want %q", body, "hello")
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/v1/buffers/default", nil)
	delReq.RemoteAddr = "127.0.0.1:12345"
	delRes := httptest.NewRecorder()
	h.ServeHTTP(delRes, delReq)
	if delRes.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want %d", delRes.Code, http.StatusNoContent)
	}
}

func TestRejectNonTailscaleRemote(t *testing.T) {
	srv := NewServer(NewStore(), 1024)
	h := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/buffers/default", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}
