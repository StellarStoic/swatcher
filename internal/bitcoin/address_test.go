// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package bitcoin

import "testing"

func TestScriptHash(t *testing.T) {
	tests := []struct{ address, want string }{
		{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "8b01df4e368ea28f8dc0423bcf7a4923e3a12d307c875e47a0cfbf90b5c39161"},
		{"bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh", ""},
	}
	for _, tc := range tests {
		got, err := ScriptHash(tc.address)
		if err != nil {
			t.Fatalf("%s: %v", tc.address, err)
		}
		if tc.want != "" && got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.address, got, tc.want)
		}
	}
}

func TestRejectsInvalidAddress(t *testing.T) {
	for _, address := range []string{"", "not-an-address", "mipcBbFg9gMiCh81Kj8tqqdgoZub1ZJRfn"} {
		if _, err := ScriptHash(address); err == nil {
			t.Fatalf("accepted %q", address)
		}
	}
}

func TestAddressFromScriptRoundTrip(t *testing.T) {
	addresses := []string{
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
		"bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
	}
	for _, address := range addresses {
		script, err := ScriptPubKey(address)
		if err != nil {
			t.Fatalf("script for %s: %v", address, err)
		}
		got, ok := AddressFromScript(script)
		if !ok || got != address {
			t.Fatalf("round trip %s: got %q, ok=%v", address, got, ok)
		}
	}
	if _, ok := AddressFromScript([]byte{0x6a, 0x01, 0x41}); ok {
		t.Fatal("OP_RETURN script unexpectedly produced an address")
	}
}
