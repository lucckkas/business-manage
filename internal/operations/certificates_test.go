package operations_test

import (
	"business-manage/internal/operations"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestRenewCertificateKeepsAuthorityAndAddsHosts(t *testing.T) {
	root := t.TempDir()
	if err := operations.GenerateCertificates(root, []string{"erp.local"}); err != nil {
		t.Fatal(err)
	}
	ca, _ := os.ReadFile(filepath.Join(root, "certificates/ca.crt"))
	if err := operations.GenerateCertificates(root, []string{"192.168.1.20"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "certificates/ca.crt"))
	if string(ca) != string(after) {
		t.Fatal("renewal replaced trusted authority")
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca)
	data, _ := os.ReadFile(filepath.Join(root, "certificates/server.crt"))
	block, _ := pem.Decode(data)
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"192.168.1.20", "localhost", "127.0.0.1"} {
		if _, err = certificate.Verify(x509.VerifyOptions{Roots: pool, DNSName: host}); err != nil {
			t.Fatal(err)
		}
	}
}
