// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package bitcoin

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestParseLegacyTransaction(t *testing.T) {
	raw := make([]byte, 0)
	raw = append(raw, 1, 0, 0, 0, 1)
	for i := byte(0); i < 32; i++ {
		raw = append(raw, i)
	}
	raw = append(raw, 2, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 1)
	value := make([]byte, 8)
	binary.LittleEndian.PutUint64(value, 5_000)
	raw = append(raw, value...)
	raw = append(raw, 3, 0x51, 0x21, 0x02, 0, 0, 0, 0)

	tx, err := ParseTransaction(hex.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.Inputs) != 1 || tx.Inputs[0].PreviousVout != 2 || tx.Inputs[0].Sequence != 0xffffffff {
		t.Fatalf("unexpected inputs: %+v", tx.Inputs)
	}
	if tx.Inputs[0].PreviousTxID != "1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100" {
		t.Fatalf("unexpected previous txid: %s", tx.Inputs[0].PreviousTxID)
	}
	if len(tx.Outputs) != 1 || tx.Outputs[0].Value != 5_000 || hex.EncodeToString(tx.Outputs[0].Script) != "512102" {
		t.Fatalf("unexpected outputs: %+v", tx.Outputs)
	}
}

func TestSignalsRBF(t *testing.T) {
	if !(Transaction{Inputs: []TxInput{{Sequence: 0xfffffffd}}}).SignalsRBF() {
		t.Fatal("transaction with sequence 0xfffffffd did not signal RBF")
	}
	for _, sequence := range []uint32{0xfffffffe, 0xffffffff} {
		if (Transaction{Inputs: []TxInput{{Sequence: sequence}}}).SignalsRBF() {
			t.Fatalf("transaction with sequence %#x incorrectly signaled RBF", sequence)
		}
	}
}

func TestParseSegwitTransaction(t *testing.T) {
	raw := "02000000000101" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"ffffffff00ffffffff01e80300000000000001510101aa00000000"
	tx, err := ParseTransaction(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.Outputs) != 1 || tx.Outputs[0].Value != 1_000 {
		t.Fatalf("unexpected transaction: %+v", tx)
	}
	if len(tx.Witnesses) != 1 || len(tx.Witnesses[0]) != 1 || hex.EncodeToString(tx.Witnesses[0][0]) != "aa" {
		t.Fatalf("unexpected witnesses: %+v", tx.Witnesses)
	}
}

func TestProtocolMarkerDetection(t *testing.T) {
	tx := Transaction{
		Outputs: []TxOutput{{Script: []byte{0x6a, 0x5d, 0x01, 0x00}}},
		Witnesses: [][][]byte{{
			{0x00, 0x63, 0x03, 'o', 'r', 'd', 0x01, 0x01, 0x68},
			{0x51, 0x00, 0x63, 0x4c, 0x03, 'o', 'r', 'd', 0x68},
		}},
	}
	if !tx.HasRunestone() {
		t.Fatal("runestone marker was not detected")
	}
	if got := tx.InscriptionEnvelopeCount(); got != 2 {
		t.Fatalf("InscriptionEnvelopeCount() = %d; want 2", got)
	}
}

func TestProtocolMarkerDetectionRejectsEmbeddedAndMalformedMarkers(t *testing.T) {
	tx := Transaction{
		Outputs: []TxOutput{{Script: []byte{0x6a, 0x01, 0x5d}}},
		Witnesses: [][][]byte{{
			{0x06, 0x00, 0x63, 0x03, 'o', 'r', 'd'},
			{0x00, 0x63, 0x04, 'o', 'r', 'd'},
		}},
	}
	if tx.HasRunestone() {
		t.Fatal("non-marker OP_RETURN was detected as a runestone")
	}
	if got := tx.InscriptionEnvelopeCount(); got != 0 {
		t.Fatalf("InscriptionEnvelopeCount() = %d; want 0", got)
	}
}

func TestParseRejectsTruncatedTransaction(t *testing.T) {
	if _, err := ParseTransaction("01000000"); err == nil {
		t.Fatal("accepted truncated transaction")
	}
}

func TestOPReturnText(t *testing.T) {
	tests := []struct {
		name   string
		script []byte
		want   string
		ok     bool
	}{
		{name: "direct push", script: append([]byte{0x6a, 11}, []byte("hello world")...), want: "hello world", ok: true},
		{name: "pushdata1", script: append([]byte{0x6a, 0x4c, 5}, []byte("hello")...), want: "hello", ok: true},
		{name: "multiple pushes", script: append(append([]byte{0x6a, 5}, []byte("hello")...), append([]byte{6}, []byte(" world")...)...), want: "hello world", ok: true},
		{name: "binary", script: []byte{0x6a, 2, 0xff, 0x00}},
		{name: "not op return", script: []byte{0x51, 1, 'a'}},
		{name: "truncated push", script: []byte{0x6a, 3, 'a'}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := OPReturnText(test.script)
			if got != test.want || ok != test.ok {
				t.Fatalf("OPReturnText() = %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}
