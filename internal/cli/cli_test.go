package cli

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServeURL(t *testing.T) {
	cases := map[string]string{
		":8080":          "http://localhost:8080/",
		"0.0.0.0:9000":   "http://0.0.0.0:9000/",
		"127.0.0.1:1234": "http://127.0.0.1:1234/",
	}
	for in, want := range cases {
		if got := ServeURL(in); got != want {
			t.Errorf("ServeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStartServer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "out.html")
	if err := os.WriteFile(file, []byte("<html>hi</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Grab a free port, then hand its address to StartServer.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	url, err := StartServer(addr, func() string { return file })
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	if want := "http://" + addr + "/"; url != want {
		t.Errorf("url = %q, want %q", url, want)
	}

	// The background goroutine may take a moment to accept connections.
	var resp *http.Response
	for i := 0; i < 50; i++ {
		resp, err = http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "<html>hi</html>" {
		t.Errorf("body = %q, want the served file contents", body)
	}
}
