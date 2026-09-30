package certificate

import (
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
	"testing"
	"time"
)

func TestLoadAndMismatch(t *testing.T) {
	dir := t.TempDir()
	makePair := func(name string) (string, string) {
		t.Helper()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "example.test"}, DNSNames: []string{"example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}, &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "example.test"}, DNSNames: []string{"example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		kb, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		cp := filepath.Join(dir, name+".crt")
		kp := filepath.Join(dir, name+".key")
		os.WriteFile(cp, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600)
		os.WriteFile(kp, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0600)
		return cp, kp
	}
	c1, k1 := makePair("one")
	_, k2 := makePair("two")
	b, err := Load(c1, k1)
	if err != nil {
		t.Fatal(err)
	}
	if len(NormalizeFingerprint(b.Fingerprint)) != 64 {
		t.Fatal("invalid fingerprint")
	}
	if _, err = Load(c1, k2); err == nil || !strings.Contains(err.Error(), "pair") {
		t.Fatalf("expected mismatch, got %v", err)
	}
}

func TestServedFingerprint(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer s.Close()
	leaf, err := x509.ParseCertificate(s.TLS.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	address := strings.TrimPrefix(s.URL, "https://")
	if _, err := ServedFingerprint(address); err == nil {
		t.Fatal("untrusted certificate was accepted")
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	got, err := servedFingerprint(address, roots)
	if err != nil {
		t.Fatal(err)
	}
	if got != Fingerprint(leaf.Raw) {
		t.Fatalf("wrong served fingerprint: %s", got)
	}
}
