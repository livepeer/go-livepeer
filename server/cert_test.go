package server

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"testing"
)

func sha1sum(fname string) ([]byte, error) {
	f, err := os.Open(fname)
	if err != nil {
		return []byte{}, err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return []byte{}, err
	}
	return h.Sum(nil), nil
}

func sha1sums(t *testing.T, cert, key string) ([]byte, []byte, error) {
	kh, err := sha1sum(key)
	if err != nil {
		t.Error("Could not sha1 keyfile", err)
		return []byte{}, []byte{}, err
	}
	ch, err := sha1sum(cert)
	if err != nil {
		t.Error("Could not sha1 certfile", err)
		return []byte{}, []byte{}, err
	}
	return ch, kh, nil
}

func TestRPCCert(t *testing.T) {
	url, _ := url.Parse("https://livepeer.org")
	wd, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Error("Could not get tempdir ", err)
		return
	}
	defer os.RemoveAll(wd)
	cf, kf, err := getCert(url, wd)
	if err != nil {
		t.Error("Could not get cert/key ", err)
		return
	}
	ch, kh, err := sha1sums(t, cf, kf)
	if err != nil {
		return
	}

	// ensure that the cert is valid and contains data we expect
	tlsCert, err := tls.LoadX509KeyPair(cf, kf)
	if err != nil {
		t.Error("Could not load cert/key pair", err)
		return
	}
	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Error("Could not parse x509 cert", err)
		return
	}
	if cert.DNSNames[0] != url.Hostname() {
		t.Error("Cert did not have expected DNS name")
		return
	}

	// ensure that when invoking again, the same cert is returned
	/* XXX re-enable when we make certs persistent
	cf, kf, err = getCert(url, wd)
	if err != nil {
		t.Error("Could not get cert/key ", err)
		return
	}
	ch1, kh1, err := sha1sums(t, cf, kf)
	if err != nil {
		return
	}
	if !bytes.Equal(kh, kh1) || !bytes.Equal(ch, ch1) {
		t.Error("Mismatched cert checksum")
		return
	}*/

	// ensure that when a cert is missing, a new key/cert is generated
	err = os.Remove(cf)
	if err != nil {
		t.Error(err)
		return
	}
	cf, kf, err = getCert(url, wd)
	if err != nil {
		t.Error("Could not get cert/key", err)
		return
	}
	ch2, kh2, err := sha1sums(t, cf, kf)
	if err != nil {
		return
	}
	if bytes.Equal(kh, kh2) || bytes.Equal(ch, ch2) {
		t.Error("Matched cert checksum")
	}

	// ensure when a key is missing, a new key/cert is generated
	err = os.Remove(kf)
	if err != nil {
		t.Error(err)
		return
	}
	cf, kf, err = getCert(url, wd)
	if err != nil {
		t.Error("Could not get cert/key", err)
		return
	}
	ch3, kh3, err := sha1sums(t, cf, kf)
	if err != nil {
		return
	}
	if bytes.Equal(kh, kh3) || bytes.Equal(ch, ch3) {
		t.Error("Matched cert checksum")
	}
}
