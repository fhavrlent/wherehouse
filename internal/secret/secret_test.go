package secret

import "testing"

func TestEncryptRoundTrip(t *testing.T) {
	key, err := LoadKey("", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	enc, err := Encrypt(key, "refresh-token-value")
	if err != nil || enc == "" || enc == "refresh-token-value" {
		t.Fatalf("encrypt: %q %v", enc, err)
	}
	if got, err := Decrypt(key, enc); err != nil || got != "refresh-token-value" {
		t.Fatalf("decrypt: %q %v", got, err)
	}
	other, _ := LoadKey("another secret", "")
	if _, err := Decrypt(other, enc); err == nil {
		t.Fatal("decrypting with the wrong key must fail")
	}
	if enc, _ := Encrypt(key, ""); enc != "" {
		t.Fatal("empty input should stay empty")
	}
}

func TestLoadKeyIsStable(t *testing.T) {
	dir := t.TempDir()
	a, err := LoadKey("", dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadKey("", dir)
	if err != nil || string(a) != string(b) {
		t.Fatal("key must be reused across restarts")
	}
}
