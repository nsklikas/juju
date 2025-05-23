// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package testing

import (
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/rand"
	"sync"

	mgotesting "github.com/juju/mgo/v3/testing"
	utilscert "github.com/juju/utils/v3/cert"
	cryptossh "golang.org/x/crypto/ssh"
)

// CACert and CAKey make up a CA key pair.
// CACertX509 and CAKeyRSA hold their parsed equivalents.
// ServerCert and ServerKey hold a CA-signed server cert/key.
// Certs holds the certificates and keys required to make a secure
// connection to a Mongo database.
var (
	once sync.Once

	CACert, CAKey, ServerCert, ServerKey = chooseGeneratedCA()

	CACertX509, CAKeyRSA = mustParseCertAndKey(CACert, CAKey)

	ServerTLSCert = mustParseServerCert(ServerCert, ServerKey)

	Certs = serverCerts()

	// Other valid test certs different from the default.
	OtherCACert, OtherCAKey        = chooseGeneratedOtherCA()
	OtherCACertX509, OtherCAKeyRSA = mustParseCertAndKey(OtherCACert, OtherCAKey)

	// SSHServerHostKey for testing
	SSHServerHostKey = mustGenerateSSHServerHostKey()
)

func chooseGeneratedCA() (string, string, string, string) {
	index := rand.Intn(len(generatedCA))
	if len(generatedCA) != len(generatedServer) {
		// This should never happen.
		panic("generatedCA and generatedServer have mismatched length")
	}
	ca := generatedCA[index]
	server := generatedServer[index]
	return ca.certPEM, ca.keyPEM, server.certPEM, server.keyPEM
}

func chooseGeneratedOtherCA() (string, string) {
	index := rand.Intn(len(otherCA))
	ca := otherCA[index]
	return ca.certPEM, ca.keyPEM
}

func mustParseServerCert(srvCert string, srvKey string) *tls.Certificate {
	tlsCert, err := tls.X509KeyPair([]byte(srvCert), []byte(srvKey))
	if err != nil {
		panic(err)
	}
	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		panic(err)
	}
	tlsCert.Leaf = x509Cert
	return &tlsCert
}

func mustParseCertAndKey(certPEM, keyPEM string) (*x509.Certificate, *rsa.PrivateKey) {
	cert, key, err := utilscert.ParseCertAndKey(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return cert, key
}

func serverCerts() *mgotesting.Certs {
	serverCert, serverKey := mustParseCertAndKey(ServerCert, ServerKey)
	return &mgotesting.Certs{
		CACert:     CACertX509,
		ServerCert: serverCert,
		ServerKey:  serverKey,
	}
}

func mustGenerateSSHServerHostKey() string {
	var k string
	once.Do(func() {
		_, privateKey, err := ed25519.GenerateKey(cryptorand.Reader)
		if err != nil {
			panic("failed to generate ED25519 key")
		}

		pemKey, err := cryptossh.MarshalPrivateKey(privateKey, "")
		if err != nil {
			panic("failed to marshal private key")
		}

		k = string(pem.EncodeToMemory(pemKey))
	})

	return k
}
