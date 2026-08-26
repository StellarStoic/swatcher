// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package bitcoin

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
)

func testXpub(t *testing.T) string {
	t.Helper()
	master, err := hdkeychain.NewMaster([]byte("s-watcher deterministic derivation test"), &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	account, err := master.Derive(hdkeychain.HardenedKeyStart)
	if err != nil {
		t.Fatal(err)
	}
	public, err := account.Neuter()
	if err != nil {
		t.Fatal(err)
	}
	return public.String()
}

func TestDerivePlainXpubBranches(t *testing.T) {
	addresses, err := DeriveAddresses(testXpub(t), "native-segwit", 3, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 6 {
		t.Fatalf("got %d addresses, want 6", len(addresses))
	}
	if addresses[0].Path != "m/0/0" || addresses[3].Path != "m/1/0" {
		t.Fatalf("unexpected paths: %+v", addresses)
	}
	for _, address := range addresses {
		if !strings.HasPrefix(address.Address, "bc1q") {
			t.Fatalf("expected native SegWit address, got %s", address.Address)
		}
	}
}

func TestPlainXpubRequiresAddressType(t *testing.T) {
	if _, err := DeriveAddresses(testXpub(t), "", 1, false); err == nil || !strings.Contains(err.Error(), "choose an address type") {
		t.Fatalf("expected an ambiguous xpub error, got %v", err)
	}
}

func TestDeriveDescriptorBranches(t *testing.T) {
	descriptor := "wpkh(" + testXpub(t) + "/<0;1>/*)"
	addresses, err := DeriveAddresses(descriptor, "legacy", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 4 || addresses[2].Path != "m/1/0" {
		t.Fatalf("unexpected descriptor derivation: %+v", addresses)
	}
}

func TestDeriveDescriptorDirectWildcard(t *testing.T) {
	descriptor := "wpkh(" + testXpub(t) + "/*)"
	addresses, err := DeriveAddresses(descriptor, "", 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 2 || addresses[0].Path != "m/0" || addresses[1].Path != "m/1" {
		t.Fatalf("unexpected direct wildcard derivation: %+v", addresses)
	}
}

func TestNormalizeYpubInfersNestedSegwit(t *testing.T) {
	payload, err := decodeBase58Check(testXpub(t))
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(payload[:4], 0x049d7cb2)
	ypub := encodeBase58Check(payload)
	addresses, err := DeriveAddresses(ypub, "legacy", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(addresses[0].Address, "3") {
		t.Fatalf("expected nested SegWit address, got %s", addresses[0].Address)
	}
}

func TestDeriveRejectsUnsafeRanges(t *testing.T) {
	if _, err := DeriveAddresses(testXpub(t), "native-segwit", 0, false); err == nil {
		t.Fatal("accepted zero derivation count")
	}
	addresses, err := DeriveAddresses(testXpub(t), "native-segwit", 501, false)
	if err != nil || len(addresses) != 501 {
		t.Fatalf("derivation remains artificially capped at 500: count=%d err=%v", len(addresses), err)
	}
	if _, err := DeriveAddresses("wpkh("+testXpub(t)+"/0'/*)", "", 1, false); err == nil {
		t.Fatal("accepted hardened public derivation")
	}
}
