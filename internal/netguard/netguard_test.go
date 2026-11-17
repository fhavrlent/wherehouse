package netguard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPublicOnly(t *testing.T) {
	for addr, ok := range map[string]bool{"1.1.1.1:443": true, "[2606:4700::1111]:443": true, "10.0.0.1:80": false,
		"192.168.1.5:80": false, "127.0.0.1:80": false, "169.254.169.254:80": false, "100.100.1.1:80": false,
		"[::1]:80": false, "[fd00::1]:80": false, "[::ffff:10.0.0.1]:80": false, "0.0.0.0:80": false} {
		if err := PublicOnly("tcp", addr, nil); (err == nil) != ok {
			t.Errorf("PublicOnly(%s) = %v", addr, err)
		}
	}
}

func TestClient(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			// The same server under a name that isn't trusted.
			http.Redirect(w, r, strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)+"/", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if res, err := Client(time.Second, "127.0.0.1").Get(srv.URL); err != nil || res.StatusCode != http.StatusNoContent {
		t.Fatalf("trusted host: %v", err)
	}
	if _, err := Client(time.Second, "127.0.0.1").Get(srv.URL + "/elsewhere"); err == nil {
		t.Error("redirect to an untrusted local name: expected an error")
	}
	if _, err := Client(time.Second).Get(srv.URL); err == nil {
		t.Error("local address with nothing trusted: expected an error")
	}
}
