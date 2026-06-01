package certutil

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

// LeafSHA256HexFromFile returns lowercase hex SHA256 of the leaf certificate DER,
// matching: openssl x509 -noout -fingerprint -sha256
func LeafSHA256HexFromFile(certPath string) (string, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("read cert file: %w", err)
	}
	return LeafSHA256HexFromPEM(data)
}

func LeafSHA256HexFromPEM(pemData []byte) (string, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return "", fmt.Errorf("decode pem failed")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse certificate: %w", err)
	}
	sum := sha256.Sum256(cert.Raw)
	return strings.ToLower(hex.EncodeToString(sum[:])), nil
}
