package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBIP322OfficialVectors(t *testing.T) {
	for _, path := range []string{"testdata/bip322-basic.json", "testdata/bip322-generated.json"} {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var vectors struct {
			Simple []struct {
				Message    string   `json:"message"`
				Address    string   `json:"address"`
				Signatures []string `json:"bip322_signatures"`
				Signature  string   `json:"signature"`
			} `json:"simple"`
		}
		if e = json.Unmarshal(b, &vectors); e != nil {
			t.Fatal(e)
		}
		count := 0
		for _, v := range vectors.Simple {
			if !strings.HasPrefix(v.Address, "bc1q") && !strings.HasPrefix(v.Address, "bc1p") {
				continue
			}
			if strings.HasPrefix(v.Address, "bc1q") && len(v.Address) != 42 {
				continue
			}
			signatures := v.Signatures
			if v.Signature != "" {
				signatures = append(signatures, v.Signature)
			}
			for _, sig := range signatures {
				count++
				if e := verifyBIP322(v.Address, v.Message, sig); e != nil {
					t.Errorf("%s %s: %v", path, v.Address, e)
				}
				if verifyBIP322(v.Address, v.Message+"tampered", sig) == nil {
					t.Fatal("tampered message accepted")
				}
			}
		}
		t.Logf("%s: %d supported official signatures", path, count)
	}
}
func TestIdentityAddressNormalization(t *testing.T) {
	a := "bc1q9vza2e8x573nczrlzms0wvx3gsqjx7vavgkx0l"
	got, e := canonicalAddress(strings.ToUpper(a))
	if e != nil || got != a {
		t.Fatalf("uppercase: %s %v", got, e)
	}
	for _, v := range []string{"bC" + a[2:], a + "x", "not-an-address"} {
		if _, e := canonicalAddress(v); e == nil {
			t.Fatalf("accepted %s", v)
		}
	}
}

func TestBIP322OfficialErrorAndTransactionVectors(t *testing.T) {
	raw, e := os.ReadFile("testdata/bip322-basic.json")
	if e != nil {
		t.Fatal(e)
	}
	var d struct {
		Errors []struct {
			Address   string `json:"address"`
			Message   string `json:"message"`
			Signature string `json:"signature"`
		} `json:"error"`
		Hashes []struct {
			Message string `json:"message"`
			Address string `json:"address"`
			Spend   string `json:"to_spend_tx_hash"`
			Sign    string `json:"to_sign_tx_hash"`
		} `json:"tx_hashes"`
	}
	if e = json.Unmarshal(raw, &d); e != nil {
		t.Fatal(e)
	}
	for _, v := range d.Errors {
		if verifyBIP322(v.Address, v.Message, v.Signature) == nil {
			t.Fatal("official invalid vector accepted")
		}
	}
	for _, v := range d.Hashes {
		spend, sign, _, e := bip322Transactions(v.Address, v.Message)
		if e != nil || spend.TxHash().String() != v.Spend || sign.TxHash().String() != v.Sign {
			t.Fatal("official BIP322 transaction hash mismatch")
		}
	}
	t.Logf("%d invalid signature vectors; %d transaction hash vectors", len(d.Errors), len(d.Hashes))
}

// A boundary witness must fail even though the dependency's older Schnorr
// parser reduces s modulo n. Regression guard for upstream btcd issue #2561.
func TestBIP322RejectsNoncanonicalSchnorrScalar(t *testing.T) {
	raw, e := os.ReadFile("testdata/bip322-generated.json")
	if e != nil {
		t.Fatal(e)
	}
	var d struct {
		Simple []struct {
			Address    string   `json:"address"`
			Message    string   `json:"message"`
			Signatures []string `json:"bip322_signatures"`
		} `json:"simple"`
	}
	json.Unmarshal(raw, &d)
	for _, v := range d.Simple {
		if !strings.HasPrefix(v.Address, "bc1p") {
			continue
		}
		witness, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(v.Signatures[0], "smp"))
		if e != nil {
			t.Fatal(e)
		}
		order, _ := hex.DecodeString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
		copy(witness[34:66], order)
		if verifyBIP322(v.Address, v.Message, base64.StdEncoding.EncodeToString(witness)) == nil {
			t.Fatal("noncanonical Schnorr scalar accepted")
		}
		return
	}
	t.Fatal("missing taproot fixture")
}
