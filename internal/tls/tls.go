package tls

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
	"net"
	"os"
	"path"
	"time"

	"go.uber.org/zap"
)

type TLSFiles[T any] struct {
	Cert    T
	PrivKey T
}

// Done for mocking purposes
var getNow = func() time.Time {
	return time.Now()
}

func getTLSFiles(tlsDir string) TLSFiles[string] {
	return TLSFiles[string]{
		Cert:    path.Join(tlsDir, "cert.pem"),
		PrivKey: path.Join(tlsDir, "key.pem"),
	}
}

func getTLSBytes(tlsDir string) (*TLSFiles[[]byte], error) {
	tlsFiles := getTLSFiles(tlsDir)

	certBytes, err := os.ReadFile(tlsFiles.Cert)
	if err != nil {
		return nil, err
	}

	privKeyBytes, err := os.ReadFile(tlsFiles.PrivKey)
	if err != nil {
		return nil, err
	}

	return &TLSFiles[[]byte]{
		Cert:    certBytes,
		PrivKey: privKeyBytes,
	}, nil
}

func serverTLSConfig(cert tls.Certificate, certPool *x509.CertPool) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}
}

func clientTLSConfig(cert tls.Certificate, certPool *x509.CertPool) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	}
}

func LoadTLSConfig(tlsDir string, isClient bool) (*tls.Config, error) {
	pemBytes, err := getTLSBytes(tlsDir)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemBytes.Cert) {
		return nil, fmt.Errorf("failed to add root certificate to pool")
	}

	cert, err := tls.X509KeyPair(pemBytes.Cert, pemBytes.PrivKey)
	if err != nil {
		return nil, err
	}

	var tlsConfig *tls.Config

	if isClient {
		tlsConfig = clientTLSConfig(cert, certPool)
	} else {
		tlsConfig = serverTLSConfig(cert, certPool)
	}

	return tlsConfig, nil
}

func GenerateTLS(tlsDir string, tlsFileMode os.FileMode, logger *zap.Logger) (*tls.Config, error) {
	if info, err := os.Stat(tlsDir); err != nil {
		if err := os.MkdirAll(tlsDir, tlsFileMode); err != nil {
			return nil, err
		}
	} else {
		if !info.IsDir() {
			return nil, fmt.Errorf("provided tls path is not a directory: %s", tlsDir)
		}

		if err := os.Chmod(tlsDir, tlsFileMode); err != nil {
			return nil, err
		}
	}

	var (
		certPath = path.Join(tlsDir, "cert.pem")
		pkPath   = path.Join(tlsDir, "key.pem")
	)

	if tlsConfig, err := LoadTLSConfig(tlsDir, false); err == nil {
		if len(tlsConfig.Certificates) < 1 || len(tlsConfig.Certificates[0].Certificate) < 1 {
			logger.Warn("invalid TLS certificate, generating new certificate...")
			goto generateNew
		}

		cert, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
		if err != nil {
			logger.Warn("failed to parse certificate, generating new certificate...", zap.Error(err))
			goto generateNew
		}

		if getNow().AddDate(0, 0, 7).After(cert.NotAfter) {
			logger.Info("pre-existing certificate will expire soon, generating new certificate...")
			goto generateNew
		}

		logger.Info("using pre-existing TLS certificate")

		return tlsConfig, nil
	}

generateNew:
	logger.Info("generating new TLS certificate...")

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "stateful",
			Organization: []string{"Stateful, INC."},
			Country:      []string{"US"},
			Province:     []string{"California"},
			Locality:     []string{"Berkeley"},
		},
		NotBefore:             getNow(),
		NotAfter:              getNow().AddDate(0, 0, 30),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.SHA256WithRSA,
		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
		},
		DNSNames: []string{
			"localhost",
		},
	}

	certificateBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, err
	}

	caPEM := new(bytes.Buffer)
	if err := pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateBytes,
	}); err != nil {
		return nil, err
	}

	privKeyPEM := new(bytes.Buffer)
	if err := pem.Encode(privKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}); err != nil {
		return nil, err
	}

	tlsCa := tls.Certificate{
		Certificate: [][]byte{certificateBytes},
		PrivateKey:  privKey,
		Leaf:        ca,
	}

	certPool := x509.NewCertPool()

	// TODO: can probably use `AddCert` here
	if !certPool.AppendCertsFromPEM(caPEM.Bytes()) {
		return nil, fmt.Errorf("failed to add certificate to certificate pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCa},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}

	if err := os.WriteFile(certPath, caPEM.Bytes(), tlsFileMode); err != nil {
		return nil, err
	}

	if err := os.WriteFile(pkPath, privKeyPEM.Bytes(), tlsFileMode); err != nil {
		return nil, err
	}

	logger.Info("successfully generated new TLS certificate!")

	return tlsConfig, nil
}
