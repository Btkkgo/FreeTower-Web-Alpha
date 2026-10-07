package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

var errSignature = errors.New("Invalid or unsupported BIP-322 signature")

func canonicalAddress(raw string) (string, error) {
	if len(raw) > 100 || strings.TrimSpace(raw) != raw {
		return "", errSignature
	}
	if strings.HasPrefix(strings.ToLower(raw), "bc1") {
		if raw != strings.ToLower(raw) && raw != strings.ToUpper(raw) {
			return "", errSignature
		}
		raw = strings.ToLower(raw)
	}
	a, e := btcutil.DecodeAddress(raw, &chaincfg.MainNetParams)
	if e != nil || !a.IsForNet(&chaincfg.MainNetParams) {
		return "", errSignature
	}
	return a.EncodeAddress(), nil
}
func bip322Transactions(address, message string) (*wire.MsgTx, *wire.MsgTx, []byte, error) {
	addr, e := btcutil.DecodeAddress(address, &chaincfg.MainNetParams)
	if e != nil {
		return nil, nil, nil, errSignature
	}
	switch addr.(type) {
	case *btcutil.AddressWitnessPubKeyHash, *btcutil.AddressTaproot:
	default:
		return nil, nil, nil, errSignature
	}
	script, e := txscript.PayToAddrScript(addr)
	if e != nil {
		return nil, nil, nil, errSignature
	}
	tag := sha256.Sum256([]byte("BIP0322-signed-message"))
	h := sha256.New()
	h.Write(tag[:])
	h.Write(tag[:])
	h.Write([]byte(message))
	digest := h.Sum(nil)
	sigScript, _ := txscript.NewScriptBuilder().AddOp(txscript.OP_0).AddData(digest).Script()
	spend := wire.NewMsgTx(0)
	in := wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0xffffffff), sigScript, nil)
	in.Sequence = 0
	spend.AddTxIn(in)
	spend.AddTxOut(wire.NewTxOut(0, script))
	spendHash := spend.TxHash()
	sign := wire.NewMsgTx(0)
	in = wire.NewTxIn(wire.NewOutPoint(&spendHash, 0), nil, nil)
	in.Sequence = 0
	sign.AddTxIn(in)
	sign.AddTxOut(wire.NewTxOut(0, []byte{txscript.OP_RETURN}))
	return spend, sign, script, nil
}

// Only native P2WPKH and P2TR key-path proofs are supported. Script execution,
// DER/low-S, ECDSA, Schnorr and Taproot tweaks are handled by btcd, not custom crypto.
func verifyBIP322(address, message, signature string) error {
	_, sign, script, e := bip322Transactions(address, message)
	if e != nil {
		return e
	}
	signature = strings.TrimPrefix(signature, "smp")
	if len(signature) > 2048 {
		return errSignature
	}
	raw, e := base64.StdEncoding.Strict().DecodeString(signature)
	if e != nil {
		return errSignature
	}
	r := bytes.NewReader(raw)
	count, e := wire.ReadVarInt(r, 0)
	if e != nil || count < 1 || count > 2 {
		return errSignature
	}
	witness := make(wire.TxWitness, count)
	for i := range witness {
		witness[i], e = wire.ReadVarBytes(r, 0, 520, "witness")
		if e != nil {
			return errSignature
		}
	}
	if r.Len() != 0 {
		return errSignature
	}
	if txscript.IsPayToWitnessPubKeyHash(script) {
		if len(witness) != 2 || len(witness[0]) < 2 || witness[0][len(witness[0])-1] != byte(txscript.SigHashAll) || len(witness[1]) != 33 {
			return errSignature
		}
	} else {
		if len(witness) != 1 || (len(witness[0]) != 64 && !(len(witness[0]) == 65 && witness[0][64] == byte(txscript.SigHashAll))) {
			return errSignature
		}
		// btcec v2.3.5 ParseSignature reduces s modulo n. Reject noncanonical
		// encoding using the library scalar parser before script execution.
		// Upstream correction: btcsuite/btcd#2561 (Go1.25-only release line).
		var scalar btcec.ModNScalar
		if scalar.SetByteSlice(witness[0][32:64]) {
			return errSignature
		}
	}
	sign.TxIn[0].Witness = witness
	fetch := txscript.NewCannedPrevOutputFetcher(script, 0)
	engine, e := txscript.NewEngine(script, sign, 0, txscript.StandardVerifyFlags, nil, txscript.NewTxSigHashes(sign, fetch), 0, fetch)
	if e != nil || engine.Execute() != nil {
		return errSignature
	}
	return nil
}
