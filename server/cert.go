package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
)

const certExpiry = 8765 * time.Hour // One year

func genCert(host string, priv *ecdsa.PrivateKey) ([]byte, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	glog.Info("Generating cert for ", host)
	if err != nil {
		glog.Error("Could not generate serial ", err)
		return []byte{}, err
	}
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(certExpiry), // XXX fix fix fix
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{host},
		BasicConstraintsValid: true,
		IsCA: true,
	}
	cert, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		glog.Error("Could not create certificate ", err)
		return []byte{}, err
	}
	return cert, nil
}

func genKey() (*ecdsa.PrivateKey, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		glog.Error("Unable to generate private key ", err)
		return nil, []byte{}, err
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		glog.Error("Unable to marshal EC private key ", err)
		return nil, []byte{}, err
	}
	return key, keyBytes, nil
}

func writeFile(fname string, desc string, contents []byte) error {
	file, err := os.Create(fname)
	defer file.Close()
	if err != nil {
		glog.Errorf("Unable to create file %v: %v", fname, err)
		return err
	}
	err = pem.Encode(file, &pem.Block{Type: desc, Bytes: contents})
	if err != nil {
		glog.Errorf("Unable to pem-encode %v: %v", fname, err)
		return err
	}
	return nil
}

func getCert(uri *url.URL, workDir string) (string, string, error) {
	// if cert doesn't exist, generate a selfsigned cert
	certFile := filepath.Join(workDir, "cert.pem")
	keyFile := filepath.Join(workDir, "key.pem")
	_, certErr := os.Stat(certFile)
	_, keyErr := os.Stat(keyFile)
	//if os.IsNotExist(certErr) || os.IsNotExist(keyErr) {
	// XXX for now, just generate a new cert every time.
	if true {
		glog.Info("Private key and cert not found. Generating")
		key, keyBytes, err := genKey()
		if err != nil {
			return "", "", err
		}
		err = writeFile(keyFile, "EC PRIVATE KEY", keyBytes)
		if err != nil {
			return "", "", err
		}
		cert, err := genCert(uri.Hostname(), key)
		if err != nil {
			return "", "", err
		}
		err = writeFile(certFile, "CERTIFICATE", cert)
		if err != nil {
			return "", "", err
		}
	} else if certErr != nil || keyErr != nil {
		glog.Error("Problem getting key/cert ", certErr, keyErr)
		err := certErr
		if keyErr != nil {
			err = keyErr
		}
		return "", "", err
	}
	return certFile, keyFile, nil
}
