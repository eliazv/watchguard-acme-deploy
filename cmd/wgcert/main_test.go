package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eliazv/watchguard-acme-deploy/internal/watchguard"
)

func TestFindExisting(t *testing.T) {
	fp := strings.Repeat("AB", 32)
	certs := []watchguard.Certificate{{ID: "crt_old", Name: "old", Fingerprint: fp}}
	got, err := findExisting(certs, fp, "new")
	if err != nil || got == nil || got.ID != "crt_old" {
		t.Fatalf("same fingerprint must be reused: %v %v", got, err)
	}
	if _, err = findExisting(certs, strings.Repeat("CD", 32), "old"); err == nil {
		t.Fatal("name collision accepted")
	}
	if _, err = findExisting(append(certs, certs[0]), fp, "new"); err == nil {
		t.Fatal("duplicate fingerprint accepted")
	}
}

func TestDryRunDoesNotMutate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "firewall.example.test"}, DNSNames: []string{"firewall.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "fullchain.pem"), filepath.Join(dir, "privkey.pem")
	if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	var mutations atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			w.Write([]byte(`{"access_token":"token"}`))
			return
		}
		if r.Method != "GET" {
			mutations.Add(1)
		}
		if strings.HasSuffix(r.URL.Path, "/devices") {
			w.Write([]byte(`{"data":[{"id":"FB-123","cloud_managed":"yes"}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/certificates") {
			w.Write([]byte(`{"objects":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer s.Close()
	t.Setenv("WATCHGUARD_ACCOUNT_ID", "ACC-1")
	t.Setenv("WATCHGUARD_API_URL", s.URL)
	t.Setenv("WATCHGUARD_AUTH_URL", s.URL)
	t.Setenv("WATCHGUARD_API_KEY", "secret")
	t.Setenv("WATCHGUARD_ACCESS_ID", "id")
	t.Setenv("WATCHGUARD_ACCESS_PASSWORD", "password")
	var out bytes.Buffer
	err = run(context.Background(), []string{"deploy", "--device", "FB-123", "--cert", certPath, "--key", keyPath, "--dry-run", "--output", "json"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if mutations.Load() != 0 || !strings.Contains(out.String(), `"action": "plan"`) {
		t.Fatalf("dry-run result: mutations=%d output=%s", mutations.Load(), out.String())
	}
	out.Reset()
	if err := run(context.Background(), []string{"check", "--device", "FB-123", "--output", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"action": "check"`) || mutations.Load() != 0 {
		t.Fatalf("check result: mutations=%d output=%s", mutations.Load(), out.String())
	}
}
