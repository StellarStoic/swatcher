// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package electrum

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
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
