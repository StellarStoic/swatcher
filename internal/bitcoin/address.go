// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package bitcoin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// ScriptHash returns the Electrum protocol scripthash: SHA256(scriptPubKey)
// with the digest byte order reversed.
func ScriptHash(address string) (string, error) {
	script, err := ScriptPubKey(strings.TrimSpace(address))
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(script)
	for i, j := 0, len(hash)-1; i < j; i, j = i+1, j-1 {
		hash[i], hash[j] = hash[j], hash[i]
	}
	return hex.EncodeToString(hash[:]), nil
}

func ScriptPubKey(address string) ([]byte, error) {
	if strings.HasPrefix(strings.ToLower(address), "bc1") {
		hrp, version, program, err := decodeSegwit(address)
		if err != nil {
			return nil, err
		}
		if hrp != "bc" {
			return nil, errors.New("only Bitcoin mainnet addresses are supported")
		}
		op := byte(0)
		if version > 0 {
			op = 0x50 + version
		}
		return append([]byte{op, byte(len(program))}, program...), nil
	}
	payload, err := decodeBase58Check(address)
	if err != nil || len(payload) != 21 {
		return nil, errors.New("invalid Bitcoin mainnet address")
	}
	switch payload[0] {
	case 0x00:
		return append(append([]byte{0x76, 0xa9, 0x14}, payload[1:]...), 0x88, 0xac), nil
	case 0x05:
		return append(append([]byte{0xa9, 0x14}, payload[1:]...), 0x87), nil
	default:
		return nil, errors.New("only Bitcoin mainnet addresses are supported")
	}
}

// AddressFromScript returns the mainnet address represented by a standard
// output script. Scripts without a conventional address, including OP_RETURN,
// return false.
func AddressFromScript(script []byte) (string, bool) {
	_, addresses, _, err := txscript.ExtractPkScriptAddrs(script, &chaincfg.MainNetParams)
	if err != nil || len(addresses) != 1 {
		return "", false
	}
	return addresses[0].EncodeAddress(), true
}

func decodeBase58Check(s string) ([]byte, error) {
	n := new(big.Int)
	base := big.NewInt(58)
	for _, r := range s {
		i := strings.IndexRune(base58Alphabet, r)
		if i < 0 {
			return nil, errors.New("invalid base58 character")
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(i)))
	}
	raw := n.Bytes()
	for len(s) > 0 && s[0] == '1' {
		raw = append([]byte{0}, raw...)
		s = s[1:]
	}
	if len(raw) < 5 {
		return nil, errors.New("base58 value too short")
	}
	payload, checksum := raw[:len(raw)-4], raw[len(raw)-4:]
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	if string(checksum) != string(second[:4]) {
		return nil, errors.New("invalid base58 checksum")
	}
	return payload, nil
}

func decodeSegwit(address string) (string, byte, []byte, error) {
	if address != strings.ToLower(address) && address != strings.ToUpper(address) {
		return "", 0, nil, errors.New("mixed-case bech32 address")
	}
	address = strings.ToLower(address)
	pos := strings.LastIndexByte(address, '1')
	if pos < 1 || pos+7 > len(address) {
		return "", 0, nil, errors.New("invalid bech32 address")
	}
	hrp := address[:pos]
	charset := "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	data := make([]byte, len(address)-pos-1)
	for i, r := range address[pos+1:] {
		v := strings.IndexRune(charset, r)
		if v < 0 {
			return "", 0, nil, errors.New("invalid bech32 character")
		}
		data[i] = byte(v)
	}
	if len(data) < 7 {
		return "", 0, nil, errors.New("invalid bech32 data")
	}
	version := data[0]
	if version > 16 {
		return "", 0, nil, errors.New("invalid witness version")
	}
	constant := uint32(1)
	if version > 0 {
		constant = 0x2bc830a3
	}
	if polymod(append(expandHRP(hrp), data...)) != constant {
		return "", 0, nil, errors.New("invalid bech32 checksum")
	}
	program, err := convertBits(data[1:len(data)-6], 5, 8, false)
	if err != nil || len(program) < 2 || len(program) > 40 {
		return "", 0, nil, errors.New("invalid witness program")
	}
	if version == 0 && len(program) != 20 && len(program) != 32 {
		return "", 0, nil, errors.New("invalid v0 witness program")
	}
	return hrp, version, program, nil
}

func expandHRP(hrp string) []byte {
	r := make([]byte, 0, len(hrp)*2+1)
	for _, c := range hrp {
		r = append(r, byte(c>>5))
	}
	r = append(r, 0)
	for _, c := range hrp {
		r = append(r, byte(c)&31)
	}
	return r
}

func polymod(values []byte) uint32 {
	chk := uint32(1)
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (top>>i)&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func convertBits(data []byte, from, to uint, pad bool) ([]byte, error) {
	var acc uint32
	var bits uint
	maxv := uint32(1<<to) - 1
	result := []byte{}
	for _, value := range data {
		if uint(value)>>from != 0 {
			return nil, fmt.Errorf("invalid %d-bit value", from)
		}
		acc = acc<<from | uint32(value)
		bits += from
		for bits >= to {
			bits -= to
			result = append(result, byte(acc>>bits&maxv))
		}
	}
	if pad {
		if bits > 0 {
			result = append(result, byte(acc<<(to-bits)&maxv))
		}
	} else if bits >= from || acc<<(to-bits)&maxv != 0 {
		return nil, errors.New("invalid padding")
	}
	return result, nil
}
