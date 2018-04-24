package record

import (
	"encoding/base64"
	"strings"
	"testing"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

var OffensiveKey = "CAASXjBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQDjXAQQMal4SB2tSnX6NJIPmC69/BT8A8jc7/gDUZNkEhdhYHvc7k7S4vntV/c92nJGxNdop9fKJyevuNMuXhhHAgMBAAE="

var badPaths = []string{
	"foo/bar/baz",
	"//foo/bar/baz",
	"/ns",
	"ns",
	"ns/",
	"",
	"//",
	"/",
	"////",
}

func TestSplitPath(t *testing.T) {
	ns, key, err := splitPath("/foo/bar/baz")
	if err != nil {
		t.Fatal(err)
	}
	if ns != "foo" {
		t.Errorf("wrong namespace: %s", ns)
	}
	if key != "bar/baz" {
		t.Errorf("wrong key: %s", key)
	}

	ns, key, err = splitPath("/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	if ns != "foo" {
		t.Errorf("wrong namespace: %s", ns)
	}
	if key != "bar" {
		t.Errorf("wrong key: %s", key)
	}

	for _, badP := range badPaths {
		_, _, err := splitPath(badP)
		if err == nil {
			t.Errorf("expected error for bad path: %s", badP)
		}
	}
}

func TestIsSigned(t *testing.T) {
	v := Validator{}
	v["sign"] = &ValidChecker{
		Sign: true,
	}
	v["nosign"] = &ValidChecker{
		Sign: false,
	}
	yes, err := v.IsSigned("/sign/a")
	if err != nil {
		t.Fatal(err)
	}
	if !yes {
		t.Error("expected ns 'sign' to be signed")
	}
	yes, err = v.IsSigned("/nosign/a")
	if err != nil {
		t.Fatal(err)
	}
	if yes {
		t.Error("expected ns 'nosign' to not be signed")
	}
	_, err = v.IsSigned("/bad/a")
	if err == nil {
		t.Error("expected ns 'bad' to return an error")
	}
	_, err = v.IsSigned("bd")
	if err == nil {
		t.Error("expected bad ns to return an error")
	}
}

func TestBadRecords(t *testing.T) {
	v := Validator{
		"pk": PublicKeyValidator,
	}

	sr := u.NewSeededRand(15) // generate deterministic keypair
	sk, pubk, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, sr)
	if err != nil {
		t.Fatal(err)
	}

	pkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	for _, badP := range badPaths {
		r, err := MakePutRecord(sk, badP, pkb, true)
		if err != nil {
			t.Fatal(err)
		}
		if v.VerifyRecord(r) == nil {
			t.Errorf("expected error for path: %s", badP)
		}
	}

	// Test missing namespace
	r, err := MakePutRecord(sk, "/missing/ns", pkb, true)
	if err != nil {
		t.Fatal(err)
	}
	if v.VerifyRecord(r) == nil {
		t.Error("expected error for missing namespace 'missing'")
	}

	// Test valid namespace
	pkh := u.Hash(pkb)
	k := "/pk/" + string(pkh)

	r, err = MakePutRecord(sk, k, pkb, true)
	if err != nil {
		t.Fatal(err)
	}

	// Sanity test.
	err = v.VerifyRecord(r)
	if err != nil {
		t.Fatal(err)
	}

	// Test invalid author error path
	r.Author = proto.String("bla")
	err = v.VerifyRecord(r)
	if err == nil {
		t.Errorf("expected error due to bad author field")
	}
}

func validatePk(k string, pkb []byte) error {
	ns, k, err := splitPath(k)
	if err != nil {
		return err
	}

	r := &ValidationRecord{Namespace: ns, Key: k, Value: pkb}
	return ValidatePublicKeyRecord(r)
}

func TestValidatePublicKey(t *testing.T) {

	pkb, err := base64.StdEncoding.DecodeString(OffensiveKey)
	if err != nil {
		t.Fatal(err)
	}

	pubk, err := ci.UnmarshalPublicKey(pkb)
	if err != nil {
		t.Fatal(err)
	}

	pkb2, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	pkh := u.Hash(pkb2)
	k := "/pk/" + string(pkh)

	// Good public key should pass
	if err := validatePk(k, pkb); err != nil {
		t.Fatal(err)
	}

	// Bad key format should fail
	var badf = "/aa/" + string(pkh)
	if err := validatePk(badf, pkb); err == nil {
		t.Fatal("Failed to detect bad prefix")
	}

	// Bad key hash should fail
	var badk = "/pk/" + strings.Repeat("A", len(pkh))
	if err := validatePk(badk, pkb); err == nil {
		t.Fatal("Failed to detect bad public key hash")
	}

	// Bad public key should fail
	pkb[0] = 'A'
	if err := validatePk(k, pkb); err == nil {
		t.Fatal("Failed to detect bad public key data")
	}
}

func TestVerifyRecordUnsigned(t *testing.T) {
	sr := u.NewSeededRand(15) // generate deterministic keypair
	sk, pubk, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, sr)
	if err != nil {
		t.Fatal(err)
	}

	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pkh := u.Hash(pubkb)
	k := "/pk/" + string(pkh)
	r, err := MakePutRecord(sk, k, pubkb, false)
	if err != nil {
		t.Fatal(err)
	}

	validator := make(Validator)
	validator["pk"] = PublicKeyValidator
	err = validator.VerifyRecord(r)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRecordSigned(t *testing.T) {
	sr := u.NewSeededRand(15) // generate deterministic keypair
	sk, pubk, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, sr)
	if err != nil {
		t.Fatal(err)
	}

	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pkh := u.Hash(pubkb)
	k := "/pk/" + string(pkh)
	r, err := MakePutRecord(sk, k, pubkb, true)
	if err != nil {
		t.Fatal(err)
	}

	var pubkValidatorWithSig = &ValidChecker{
		Func: ValidatePublicKeyRecord,
		Sign: true,
	}
	validator := make(Validator)
	validator["pk"] = pubkValidatorWithSig
	err = validator.VerifyRecord(r)
	if err != nil {
		t.Fatal(err)
	}

	err = CheckRecordSig(r, pubk)
	if err != nil {
		t.Fatal(err)
	}

	// New Public Key
	_, pubk2, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, u.NewSeededRand(20))
	if err != nil {
		t.Fatal(err)
	}

	// Check against wrong public key.
	err = CheckRecordSig(r, pubk2)
	if err == nil {
		t.Error("signature should not validate with bad key")
	}

	// Corrupt record.
	r.Value[0] = 1

	// Check bad data against correct key
	err = CheckRecordSig(r, pubk)
	if err == nil {
		t.Error("signature should not validate with bad data")
	}
}
