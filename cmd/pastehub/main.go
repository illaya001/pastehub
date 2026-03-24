package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"pastehub/internal/pastehub"
)

const defaultBuffer = "default"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		must(runServe(os.Args[2:]))
	case "set":
		must(runSet(os.Args[2:]))
	case "get":
		must(runGet(os.Args[2:]))
	case "list":
		must(runList(os.Args[2:]))
	case "delete":
		must(runDelete(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `pastehub

Usage:
  pastehub serve [--listen addr] [--max-bytes n]
  pastehub set [buffer] [--file path | --text value]
  pastehub get [buffer] [--out path]
  pastehub list
  pastehub delete [buffer]

Client flags:
  --server URL   Server base URL (default: http://127.0.0.1:8080)

If no buffer is given, the client uses %q.
`, defaultBuffer)
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := fs.String("listen", ":8080", "listen address")
	maxBytes := fs.Int64("max-bytes", 1024*1024, "maximum payload size in bytes")
	readTimeout := fs.Duration("read-timeout", 15*time.Second, "HTTP read timeout")
	writeTimeout := fs.Duration("write-timeout", 15*time.Second, "HTTP write timeout")
	idleTimeout := fs.Duration("idle-timeout", 60*time.Second, "HTTP idle timeout")
	fs.Parse(args)

	store := pastehub.NewStore()
	handler := pastehub.NewServer(store, *maxBytes).Handler()

	srv := &http.Server{
		Addr:         *listen,
		Handler:      handler,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
		IdleTimeout:  *idleTimeout,
	}

	log.Printf("pastehub listening on %s (max-bytes=%d)", *listen, *maxBytes)
	return srv.ListenAndServe()
}

func runSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	serverURL := fs.String("server", envOrDefault("PASTEHUB_SERVER", "http://127.0.0.1:8080"), "server base URL")
	filePath := fs.String("file", "", "read payload from file")
	text := fs.String("text", "", "use text payload directly")
	contentType := fs.String("type", "application/octet-stream", "payload content type")
	itemName := fs.String("name", "", "optional item name")
	fs.Parse(args)

	buffer := defaultBuffer
	if fs.NArg() > 0 {
		buffer = fs.Arg(0)
	}

	var data []byte
	var err error
	switch {
	case *filePath != "" && *text != "":
		return errors.New("only one of --file or --text may be used")
	case *filePath != "":
		data, err = os.ReadFile(*filePath)
		if err != nil {
			return err
		}
		if *itemName == "" {
			*itemName = path.Base(*filePath)
		}
	case *text != "":
		data = []byte(*text)
		if *contentType == "application/octet-stream" {
			*contentType = "text/plain; charset=utf-8"
		}
	default:
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	}

	endpoint, err := bufferURL(*serverURL, buffer)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", *contentType)
	if *itemName != "" {
		req.Header.Set("X-Item-Name", *itemName)
	}

	resp, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	fmt.Fprintf(os.Stderr, "stored buffer %q (%s bytes=%s)\n", buffer, resp.Header.Get("Content-Type"), resp.Header.Get("X-Item-Size"))
	return nil
}

func runGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	serverURL := fs.String("server", envOrDefault("PASTEHUB_SERVER", "http://127.0.0.1:8080"), "server base URL")
	outPath := fs.String("out", "", "write payload to file instead of stdout")
	fs.Parse(args)

	buffer := defaultBuffer
	if fs.NArg() > 0 {
		buffer = fs.Arg(0)
	}

	endpoint, err := bufferURL(*serverURL, buffer)
	if err != nil {
		return err
	}

	resp, err := httpClient().Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}

	var out io.Writer = os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	serverURL := fs.String("server", envOrDefault("PASTEHUB_SERVER", "http://127.0.0.1:8080"), "server base URL")
	fs.Parse(args)

	endpoint, err := joinURL(*serverURL, "/v1/buffers")
	if err != nil {
		return err
	}

	resp, err := httpClient().Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}

	var items []struct {
		BufferName  string    `json:"buffer_name"`
		ItemName    string    `json:"item_name,omitempty"`
		ContentType string    `json:"content_type"`
		Size        int       `json:"size"`
		SHA256      string    `json:"sha256"`
		CreatedAt   time.Time `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return err
	}

	for _, item := range items {
		name := item.ItemName
		if name == "" {
			name = "-"
		}
		fmt.Printf("%s\t%d\t%s\t%s\t%s\n", item.BufferName, item.Size, item.ContentType, name, item.CreatedAt.Format(time.RFC3339))
	}
	return nil
}

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	serverURL := fs.String("server", envOrDefault("PASTEHUB_SERVER", "http://127.0.0.1:8080"), "server base URL")
	fs.Parse(args)

	buffer := defaultBuffer
	if fs.NArg() > 0 {
		buffer = fs.Arg(0)
	}

	endpoint, err := bufferURL(*serverURL, buffer)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	fmt.Fprintf(os.Stderr, "deleted buffer %q\n", buffer)
	return nil
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func bufferURL(baseURL, buffer string) (string, error) {
	return joinURL(baseURL, "/v1/buffers/"+url.PathEscape(buffer))
}

func joinURL(baseURL, suffix string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + suffix
	return u.String(), nil
}

func responseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("request failed: %s: %s", resp.Status, msg)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func must(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
