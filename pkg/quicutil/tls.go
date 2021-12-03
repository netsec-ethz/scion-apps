// Copyright 2021 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package quicutil

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// MustGenerateSelfSignedCert generates private key and a self-signed dummy
// certificate usable for TLS with InsecureSkipVerify: true.
// Like GenerateSelfSignedCert but panics on error and returns a slice with a
// single entry, for convenience when initializing a tls.Config structure.
func MustGenerateSelfSignedCert() []tls.Certificate {
	cert, err := GenerateSelfSignedCert()
	if err != nil {
		panic(err)
	}
	return []tls.Certificate{*cert}
}

// GenerateSelfSignedCert generates a private key and a self-signed dummy
// certificate usable for TLS with InsecureSkipVerify: true
func GenerateSelfSignedCert() (*tls.Certificate, error) {
	priv, err := rsaGenerateKey()
	if err != nil {
		return nil, err
	}
	return createCertificate(priv)
}

func rsaGenerateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// createCertificate creates a self-signed dummy certificate for the given key
// Inspired/copy pasted from crypto/tls/generate_cert.go
func createCertificate(priv *rsa.PrivateKey) (*tls.Certificate, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"scionlab"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"dummy"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEMBuf := &bytes.Buffer{}
	if err := pem.Encode(certPEMBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, err
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal private key: %w", err)
	}

	keyPEMBuf := &bytes.Buffer{}
	if err := pem.Encode(keyPEMBuf, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPEMBuf.Bytes(), keyPEMBuf.Bytes())
	return &cert, err
}
