package certificate

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type Bundle struct {
	Leaf            *x509.Certificate
	CertPEM, KeyPEM string
	Fingerprint     string
}

func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	s := strings.ToUpper(hex.EncodeToString(sum[:]))
	parts := make([]string, 0, 32)
	for i := 0; i < len(s); i += 2 {
		parts = append(parts, s[i:i+2])
	}
	return strings.Join(parts, ":")
}
func NormalizeFingerprint(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), ":", ""), "-", ""))
}

func Load(certPath, keyPath string) (Bundle, error) {
	var b Bundle
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return b, fmt.Errorf("read certificate: %w", err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return b, fmt.Errorf("read private key: %w", err)
	}
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return b, fmt.Errorf("invalid certificate/private key pair: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return b, fmt.Errorf("parse certificate: %w", err)
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return b, errors.New("certificate is not currently valid")
	}
	if len(leaf.DNSNames) == 0 && len(leaf.IPAddresses) == 0 {
		return b, errors.New("certificate has no subject alternative names")
	}
	// The full chain must be sent, and key material is kept only in memory.
	if block, _ := pem.Decode(cert); block == nil || block.Type != "CERTIFICATE" {
		return b, errors.New("certificate is not PEM encoded")
	}
	return Bundle{leaf, string(cert), string(key), Fingerprint(leaf.Raw)}, nil
}

func ServedFingerprint(address string) (string, error) {
	return servedFingerprint(address, nil)
}

func servedFingerprint(address string, roots *x509.CertPool) (string, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("verify-host requires host:port: %w", err)
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", address, &tls.Config{ServerName: host, RootCAs: roots, MinVersion: tls.VersionTLS12})
	if err != nil {
		return "", fmt.Errorf("TLS verification failed: %w", err)
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", errors.New("TLS peer sent no certificate")
	}
	return Fingerprint(certs[0].Raw), nil
}
