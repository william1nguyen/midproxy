package proxy_test

import (
	"crypto/x509"
	"testing"

	"github.com/william1nguyen/midproxy/internal/proxy"
)

func TestNewCertStoreCreatesCA(t *testing.T) {
	cs, err := proxy.NewCertStore()
	if err != nil {
		t.Fatal(err)
	}
	if cs.CA == nil {
		t.Fatal("CA is nil")
	}
	if !cs.CA.IsCA {
		t.Error("expected IsCA == true")
	}
}

func TestGetCertForNewHost(t *testing.T) {
	cs, err := proxy.NewCertStore()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := cs.Get("example.com")
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(cert.Certificate[0])
	if err := leaf.CheckSignatureFrom(cs.CA); err != nil {
		t.Errorf("cert not signed by CA: %v", err)
	}
}

func TestGetCertCacheHit(t *testing.T) {
	cs, _ := proxy.NewCertStore()
	cert1, _ := cs.Get("example.com")
	cert2, _ := cs.Get("example.com")
	if cert1 != cert2 {
		t.Error("expected same cert pointer on cache hit")
	}
}

func TestCertHasCorrectSAN(t *testing.T) {
	cs, _ := proxy.NewCertStore()
	cert, _ := cs.Get("foo.com")
	leaf, _ := x509.ParseCertificate(cert.Certificate[0])
	if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != "foo.com" {
		t.Errorf("DNSNames = %v, want [foo.com]", leaf.DNSNames)
	}
}
