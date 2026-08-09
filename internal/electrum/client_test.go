// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package electrum

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/s-watcher/s-watcher/internal/bitcoin"
)

func TestBlockTimeFromHeader(t *testing.T) {
	header := make([]byte, 80)
	binary.LittleEndian.PutUint32(header[68:72], 1_700_000_000)
	got, err := blockTimeFromHeader(hex.EncodeToString(header))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Unix(1_700_000_000, 0).UTC(); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
	if _, err := blockTimeFromHeader("00"); err == nil {
		t.Fatal("short block header was accepted")
	}
}

func TestAddInputEffectResolvesExternalSourceAddress(t *testing.T) {
	const sourceAddress = "bc1qnk4zh9qcnap2mycp56qjrgza3cc8ylrh8fecp0"
	script, err := bitcoin.ScriptPubKey(sourceAddress)
	if err != nil {
		t.Fatal(err)
	}
	effect := Effect{}
	output := bitcoin.TxOutput{Value: 42_000, Script: script}
	addInputEffect(&effect, output, map[string]bool{})
	addInputEffect(&effect, output, map[string]bool{})
	if len(effect.SourceAddresses) != 1 || effect.SourceAddresses[0] != sourceAddress {
		t.Fatalf("unexpected source addresses: %#v", effect.SourceAddresses)
	}
	if effect.SourceAddressAmounts[sourceAddress] != 84_000 {
		t.Fatalf("source inputs were not summed: %#v", effect.SourceAddressAmounts)
	}
	if effect.Sent != 0 || len(effect.SpentScripts) != 0 {
		t.Fatalf("external input was counted as watched spending: %+v", effect)
	}
}

func TestAddInputEffectCountsWatchedInputWithoutSourceLabel(t *testing.T) {
	const watchedAddress = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	script, err := bitcoin.ScriptPubKey(watchedAddress)
	if err != nil {
		t.Fatal(err)
	}
	effect := Effect{}
	addInputEffect(&effect, bitcoin.TxOutput{Value: 21_000, Script: script}, map[string]bool{string(script): true})
	if effect.Sent != 21_000 || len(effect.SpentScripts) != 1 || effect.SpentScriptAmounts[string(script)] != 21_000 || len(effect.SourceAddresses) != 0 {
		t.Fatalf("unexpected watched input effect: %+v", effect)
	}
}

func TestAddAmountSumsRepeatedOutputs(t *testing.T) {
	amounts := map[string]uint64{}
	addAmount(&amounts, "address", 12_000)
	addAmount(&amounts, "address", 345)
	if amounts["address"] != 12_345 {
		t.Fatalf("repeated output amounts were not summed: %#v", amounts)
	}
}
