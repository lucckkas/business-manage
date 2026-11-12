package operations

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func GenerateCertificates(dir string, hosts []string) error {
	dir = filepath.Join(dir, "certificates")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	caKey, ca, caDER, err := loadOrCreateAuthority(dir)
	if err != nil {
		return err
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial := func() *big.Int {
		n, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if e != nil {
			panic(e)
		}
		return n
	}
	now := time.Now()
	server := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "Raíz ERP"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if server.NotAfter.After(ca.NotAfter) {
		server.NotAfter = ca.NotAfter
	}
	if time.Until(server.NotAfter) < 24*time.Hour {
		return fmt.Errorf("la autoridad local está próxima a vencer; requiere reemplazo planificado y distribución de confianza")
	}
	hostname, _ := os.Hostname()
	hosts = append(hosts, "localhost", "127.0.0.1", "::1", hostname)
	seen := map[string]bool{}
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		if ip := net.ParseIP(host); ip != nil {
			server.IPAddresses = append(server.IPAddresses, ip)
		} else {
			server.DNSNames = append(server.DNSNames, host)
		}
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, server, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	write := func(name, kind string, data []byte) error {
		return os.WriteFile(filepath.Join(dir, name), pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: data}), 0600)
	}
	for _, item := range []struct {
		name string
		key  *ecdsa.PrivateKey
	}{{"ca.key", caKey}, {"server.key", serverKey}} {
		encoded, e := x509.MarshalPKCS8PrivateKey(item.key)
		if e != nil {
			return e
		}
		if e = write(item.name, "PRIVATE KEY", encoded); e != nil {
			return e
		}
	}
	if err = write("ca.crt", "CERTIFICATE", caDER); err != nil {
		return err
	}
	return write("server.crt", "CERTIFICATE", serverDER)
}

func loadOrCreateAuthority(dir string) (*ecdsa.PrivateKey, *x509.Certificate, []byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, nil, nil, fmt.Errorf("certificado de autoridad inválido")
		}
		ca, e := x509.ParseCertificate(block.Bytes)
		if e != nil {
			return nil, nil, nil, e
		}
		keyData, e := os.ReadFile(filepath.Join(dir, "ca.key"))
		if e != nil {
			return nil, nil, nil, e
		}
		keyBlock, _ := pem.Decode(keyData)
		if keyBlock == nil {
			return nil, nil, nil, fmt.Errorf("clave de autoridad inválida")
		}
		parsed, e := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if e != nil {
			return nil, nil, nil, e
		}
		key, ok := parsed.(*ecdsa.PrivateKey)
		public, publicOK := ca.PublicKey.(*ecdsa.PublicKey)
		if !ok || !publicOK || !ca.IsCA || !public.Equal(&key.PublicKey) {
			return nil, nil, nil, fmt.Errorf("la clave no corresponde a la autoridad local")
		}
		return key, ca, block.Bytes, nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Raíz ERP · Autoridad local"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	return key, ca, der, err
}
