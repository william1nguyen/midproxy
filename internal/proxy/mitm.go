package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"sync"
	"time"
)

type certStore struct {
	ca    *x509.Certificate
	caKey *ecdsa.PrivateKey
	mu    sync.RWMutex
	cache map[string]*tls.Certificate
}

func newCertStore() (*certStore, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Midproxy CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	ca, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}

	return &certStore{ca: ca, caKey: key, cache: make(map[string]*tls.Certificate)}, nil
}

func (cs *certStore) get(host string) (*tls.Certificate, error) {
	cs.mu.RLock()
	if c, ok := cs.cache[host]; ok {
		cs.mu.RUnlock()
		return c, nil
	}
	cs.mu.RUnlock()

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if c, ok := cs.cache[host]; ok {
		return c, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{host},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, cs.ca, &key.PublicKey, cs.caKey)
	if err != nil {
		return nil, err
	}

	cert := &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
	}
	cs.cache[host] = cert
	return cert, nil
}
