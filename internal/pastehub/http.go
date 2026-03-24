package pastehub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

type Server struct {
	store    *Store
	maxBytes int64
}

func NewServer(store *Store, maxBytes int64) *Server {
	return &Server{store: store, maxBytes: maxBytes}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/buffers", s.handleBuffers)
	mux.HandleFunc("/v1/buffers/", s.handleBuffer)
	return tailscaleOnly(mux)
}

func tailscaleOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedRemoteIP(r.RemoteAddr) {
			http.Error(w, "access allowed only from loopback or Tailscale IPs", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedRemoteIP(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	return isTailscaleIPv4(ip) || isTailscaleIPv6(ip)
}

func isTailscaleIPv4(ip net.IP) bool {
	_, cidr, _ := net.ParseCIDR("100.64.0.0/10")
	return cidr.Contains(ip)
}

func isTailscaleIPv6(ip net.IP) bool {
	_, cidr, _ := net.ParseCIDR("fd7a:115c:a1e0::/48")
	return cidr.Contains(ip)
}

func (s *Server) handleBuffers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/buffers" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.store.List())
}

func (s *Server) handleBuffer(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/buffers/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) > 2 {
		http.NotFound(w, r)
		return
	}
	bufferName := strings.TrimSpace(parts[0])
	if bufferName == "" {
		http.Error(w, "buffer name is required", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 {
		if parts[1] != "meta" {
			http.NotFound(w, r)
			return
		}
		s.handleMeta(w, r, bufferName)
		return
	}

	switch r.Method {
	case http.MethodPut:
		s.putBuffer(w, r, bufferName)
	case http.MethodGet:
		s.getBuffer(w, r, bufferName, true)
	case http.MethodHead:
		s.getBuffer(w, r, bufferName, false)
	case http.MethodDelete:
		s.deleteBuffer(w, r, bufferName)
	default:
		methodNotAllowed(w, http.MethodPut, http.MethodGet, http.MethodHead, http.MethodDelete)
	}
}

func (s *Server) putBuffer(w http.ResponseWriter, r *http.Request, bufferName string) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBytes)
	defer r.Body.Close()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, fmt.Sprintf("payload exceeds maximum size of %d bytes", s.maxBytes), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}

	item := s.store.Put(bufferName, r.Header.Get("X-Item-Name"), r.Header.Get("Content-Type"), data)
	writeItemHeaders(w, item)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) getBuffer(w http.ResponseWriter, r *http.Request, bufferName string, includeBody bool) {
	item, ok := s.store.Get(bufferName)
	if !ok {
		http.Error(w, "buffer not found", http.StatusNotFound)
		return
	}
	writeItemHeaders(w, item)
	if !includeBody {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(item.Data)
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request, bufferName string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	item, ok := s.store.Get(bufferName)
	if !ok {
		http.Error(w, "buffer not found", http.StatusNotFound)
		return
	}
	item.Data = nil
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteBuffer(w http.ResponseWriter, r *http.Request, bufferName string) {
	if !s.store.Delete(bufferName) {
		http.Error(w, "buffer not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeItemHeaders(w http.ResponseWriter, item Item) {
	header := w.Header()
	header.Set("Content-Type", item.ContentType)
	header.Set("Content-Length", fmt.Sprintf("%d", item.Size))
	header.Set("ETag", fmt.Sprintf("\"%s\"", item.SHA256))
	header.Set("X-Buffer-Name", item.BufferName)
	header.Set("X-Item-Size", fmt.Sprintf("%d", item.Size))
	header.Set("X-Item-SHA256", item.SHA256)
	header.Set("X-Created-At", item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	if item.ItemName != "" {
		header.Set("X-Item-Name", item.ItemName)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
