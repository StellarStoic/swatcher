// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package bitcoin

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

type DerivedAddress struct {
	Address string
	Path    string
}

type deriveSpec struct {
	key        string
	scriptType string
	branches   []uint32
	prefix     []uint32
}

const noBranch = ^uint32(0)

var descriptorPattern = regexp.MustCompile(`^(wpkh|pkh|tr|sh\(wpkh)\((.+)\)(\)?)$`)

// DeriveAddresses derives a bounded set of public addresses. Plain extended
// keys use /0/* and optionally /1/*. Descriptors may specify a non-hardened
// suffix ending in /* and may use <0;1> for both wallet branches.
func DeriveAddresses(input, scriptType string, count int, includeChange bool) ([]DerivedAddress, error) {
	if count < 1 {
		return nil, errors.New("derivation count must be positive")
	}
	spec, err := parseDeriveSpec(strings.TrimSpace(input), scriptType, includeChange)
	if err != nil {
		return nil, err
	}
	normalized, inferredType, err := normalizeExtendedKey(spec.key)
	if err != nil {
		return nil, err
	}
	// Descriptor wrappers are authoritative. For plain SLIP-132 keys, the key
	// version is authoritative. A plain xpub has no script semantics and must be
	// paired with an explicit selection.
	if !strings.Contains(strings.TrimSpace(input), "(") && inferredType != "" {
		spec.scriptType = inferredType
	} else if spec.scriptType == "" {
		spec.scriptType = inferredType
	}
	if spec.scriptType == "" {
		return nil, errors.New("choose an address type for this xpub")
	}
	key, err := hdkeychain.NewKeyFromString(normalized)
	if err != nil || key.IsPrivate() {
		return nil, errors.New("invalid public extended key")
	}
	results := make([]DerivedAddress, 0, count*len(spec.branches))
	for _, branch := range spec.branches {
		branchKey := key
		parts := append([]uint32(nil), spec.prefix...)
		if branch != noBranch {
			parts = append(parts, branch)
		}
		for _, part := range parts {
			branchKey, err = branchKey.Derive(part)
			if err != nil {
				return nil, fmt.Errorf("derive public path component %d: %w", part, err)
			}
		}
		for index := 0; index < count; index++ {
			child, err := branchKey.Derive(uint32(index))
			if err != nil {
				return nil, fmt.Errorf("derive address %d: %w", index, err)
			}
			address, err := addressForKey(child, spec.scriptType)
			if err != nil {
				return nil, err
			}
			pathParts := append(append([]uint32(nil), parts...), uint32(index))
			path := "m"
			for _, part := range pathParts {
				path += "/" + strconv.FormatUint(uint64(part), 10)
			}
			results = append(results, DerivedAddress{Address: address, Path: path})
		}
	}
	return results, nil
}

func parseDeriveSpec(input, selectedType string, includeChange bool) (deriveSpec, error) {
	if input == "" {
		return deriveSpec{}, errors.New("address, xpub, or descriptor is required")
	}
	if !strings.Contains(input, "(") {
		branches := []uint32{0}
		if includeChange {
			branches = append(branches, 1)
		}
		return deriveSpec{key: input, scriptType: selectedType, branches: branches}, nil
	}
	input = strings.SplitN(input, "#", 2)[0]
	match := descriptorPattern.FindStringSubmatch(input)
	if match == nil || (match[1] == "sh(wpkh" && match[3] != ")") || (match[1] != "sh(wpkh" && match[3] != "") {
		return deriveSpec{}, errors.New("supported descriptors are pkh(), wpkh(), sh(wpkh()), and tr()")
	}
	typeName := map[string]string{"pkh": "legacy", "wpkh": "native-segwit", "sh(wpkh": "nested-segwit", "tr": "taproot"}[match[1]]
	keyExpression := match[2]
	if strings.HasPrefix(keyExpression, "[") {
		end := strings.IndexByte(keyExpression, ']')
		if end < 0 {
			return deriveSpec{}, errors.New("descriptor key origin is not closed")
		}
		keyExpression = keyExpression[end+1:]
	}
	star := strings.LastIndex(keyExpression, "/*")
	if star < 0 || star+2 != len(keyExpression) {
		return deriveSpec{}, errors.New("descriptor must end with /*")
	}
	baseAndPath := keyExpression[:star]
	slash := strings.IndexByte(baseAndPath, '/')
	key := baseAndPath
	path := ""
	if slash >= 0 {
		key, path = baseAndPath[:slash], baseAndPath[slash+1:]
	}
	parts := []string{}
	if path != "" {
		parts = strings.Split(path, "/")
	}
	branches := []uint32{noBranch}
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		switch last {
		case "<0;1>", "<1;0>":
			branches = []uint32{0, 1}
			parts = parts[:len(parts)-1]
		default:
			value, err := publicPathComponent(last)
			if err != nil {
				return deriveSpec{}, err
			}
			branches = []uint32{value}
			parts = parts[:len(parts)-1]
		}
	}
	prefix := make([]uint32, 0, len(parts))
	for _, part := range parts {
		value, err := publicPathComponent(part)
		if err != nil {
			return deriveSpec{}, err
		}
		prefix = append(prefix, value)
	}
	return deriveSpec{key: key, scriptType: typeName, branches: branches, prefix: prefix}, nil
}

func publicPathComponent(value string) (uint32, error) {
	if strings.HasSuffix(value, "'") || strings.HasSuffix(value, "h") {
		return 0, errors.New("cannot derive hardened descriptor paths from an xpub")
	}
	n, err := strconv.ParseUint(value, 10, 31)
	if err != nil {
		return 0, fmt.Errorf("invalid descriptor path component %q", value)
	}
	return uint32(n), nil
}

func normalizeExtendedKey(key string) (string, string, error) {
	payload, err := decodeBase58Check(key)
	if err != nil || len(payload) != 78 {
		return "", "", errors.New("invalid extended public key")
	}
	versions := map[uint32]string{
		0x0488b21e: "",              // xpub does not encode script semantics.
		0x049d7cb2: "nested-segwit", // ypub
		0x04b24746: "native-segwit", // zpub
	}
	version := binary.BigEndian.Uint32(payload[:4])
	inferred, ok := versions[version]
	if !ok {
		return "", "", errors.New("only mainnet xpub, ypub, and zpub keys are supported")
	}
	copy(payload[:4], []byte{0x04, 0x88, 0xb2, 0x1e})
	return encodeBase58Check(payload), inferred, nil
}

func addressForKey(key *hdkeychain.ExtendedKey, scriptType string) (string, error) {
	pub, err := key.ECPubKey()
	if err != nil {
		return "", fmt.Errorf("read derived public key: %w", err)
	}
	hash := btcutil.Hash160(pub.SerializeCompressed())
	var address btcutil.Address
	switch scriptType {
	case "legacy":
		address, err = btcutil.NewAddressPubKeyHash(hash, &chaincfg.MainNetParams)
	case "nested-segwit":
		witness, witnessErr := btcutil.NewAddressWitnessPubKeyHash(hash, &chaincfg.MainNetParams)
		if witnessErr != nil {
			return "", witnessErr
		}
		redeem, scriptErr := txscript.PayToAddrScript(witness)
		if scriptErr != nil {
			return "", scriptErr
		}
		address, err = btcutil.NewAddressScriptHash(redeem, &chaincfg.MainNetParams)
	case "native-segwit", "":
		address, err = btcutil.NewAddressWitnessPubKeyHash(hash, &chaincfg.MainNetParams)
	case "taproot":
		outputKey := txscript.ComputeTaprootKeyNoScript(pub)
		address, err = btcutil.NewAddressTaproot(schnorr.SerializePubKey(outputKey), &chaincfg.MainNetParams)
	default:
		return "", fmt.Errorf("unsupported script type %q", scriptType)
	}
	if err != nil {
		return "", fmt.Errorf("create derived address: %w", err)
	}
	return address.EncodeAddress(), nil
}

func encodeBase58Check(payload []byte) string {
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	raw := append(append([]byte(nil), payload...), second[:4]...)
	zeroes := 0
	for zeroes < len(raw) && raw[zeroes] == 0 {
		zeroes++
	}
	n := new(big.Int).SetBytes(raw)
	base := big.NewInt(58)
	var encoded []byte
	mod := new(big.Int)
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		encoded = append(encoded, base58Alphabet[mod.Int64()])
	}
	for i := 0; i < zeroes; i++ {
		encoded = append(encoded, '1')
	}
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}
	return string(bytes.Clone(encoded))
}
