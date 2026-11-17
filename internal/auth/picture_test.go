package auth

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchPicture(t *testing.T) {
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	bodies := map[string][]byte{
		"/png":   pngData.Bytes(),
		"/svg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"/large": append(append([]byte{}, pngData.Bytes()...), make([]byte, maxPictureSize)...),
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			// The same server under another name, which isn't trusted.
			http.Redirect(w, r, strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)+"/png", http.StatusFound)
			return
		}
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	}))
	defer srv.Close()

	// The test server stands in for the identity provider's host.
	const trusted = "127.0.0.1"
	data, contentType, err := fetchPicture(context.Background(), trusted, srv.URL+"/png")
	if err != nil || contentType != "image/png" || !bytes.Equal(data, pngData.Bytes()) {
		t.Fatalf("png: got %q, %v", contentType, err)
	}
	for _, url := range []string{srv.URL + "/svg", srv.URL + "/large", srv.URL + "/missing", srv.URL + "/elsewhere",
		"file:///etc/passwd", "javascript:alert(1)"} {
		if _, _, err := fetchPicture(context.Background(), trusted, url); err == nil {
			t.Errorf("%s: expected an error", url)
		}
	}
	// Not from the provider's host, a local address is refused.
	if _, _, err := fetchPicture(context.Background(), "idp.example.com", srv.URL+"/png"); err == nil {
		t.Error("local address from an untrusted host: expected an error")
	}
}
