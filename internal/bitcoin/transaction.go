// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package bitcoin

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Transaction struct {
	Inputs    []TxInput
	Outputs   []TxOutput
	Witnesses [][][]byte
}

type TxInput struct {
	PreviousTxID string
	PreviousVout uint32
	Sequence     uint32
}

type TxOutput struct {
	Value  uint64
	Script []byte
}

// SignalsRBF reports explicit opt-in Replace-by-Fee signaling as defined by
// BIP125: at least one input has an nSequence below 0xfffffffe.
func (tx Transaction) SignalsRBF() bool {
	for _, input := range tx.Inputs {
		if input.Sequence < 0xfffffffe {
			return true
		}
	}
	return false
}

// HasRunestone reports whether an output contains the Runes protocol marker.
// It intentionally does not decode or interpret the runestone payload.
func (tx Transaction) HasRunestone() bool {
	for _, output := range tx.Outputs {
		if len(output.Script) >= 2 && output.Script[0] == 0x6a && output.Script[1] == 0x5d {
			return true
		}
	}
	return false
}

// InscriptionEnvelopeCount counts Ordinals protocol envelopes in witness
// scripts. Content and metadata are deliberately neither decoded nor stored.
func (tx Transaction) InscriptionEnvelopeCount() int {
	count := 0
	for _, witness := range tx.Witnesses {
		for _, item := range witness {
			count += inscriptionEnvelopes(item)
		}
	}
	return count
}

func inscriptionEnvelopes(script []byte) int {
	count := 0
	state := 0
	for position := 0; position < len(script); {
		opcode, pushed, data, next, ok := scriptOperation(script, position)
		if !ok {
			return count
		}
		position = next
		switch state {
		case 0:
			if opcode == 0x00 {
				state = 1
			}
		case 1:
			if opcode == 0x63 {
				state = 2
			} else if opcode != 0x00 {
				state = 0
			}
		case 2:
			if pushed && string(data) == "ord" {
				count++
			}
			if opcode == 0x00 {
				state = 1
			} else {
				state = 0
			}
		}
	}
	return count
}

func scriptOperation(script []byte, position int) (byte, bool, []byte, int, bool) {
	if position >= len(script) {
		return 0, false, nil, position, false
	}
	opcode := script[position]
	position++
	var size uint64
	switch {
	case opcode == 0x00:
		return opcode, true, nil, position, true
	case opcode <= 75:
		size = uint64(opcode)
	case opcode == 0x4c:
		if position >= len(script) {
			return 0, false, nil, position, false
		}
		size = uint64(script[position])
		position++
	case opcode == 0x4d:
		if position+2 > len(script) {
			return 0, false, nil, position, false
		}
		size = uint64(binary.LittleEndian.Uint16(script[position : position+2]))
		position += 2
	case opcode == 0x4e:
		if position+4 > len(script) {
			return 0, false, nil, position, false
		}
		size = uint64(binary.LittleEndian.Uint32(script[position : position+4]))
		position += 4
	default:
		return opcode, false, nil, position, true
	}
	if size > uint64(len(script)-position) {
		return 0, false, nil, position, false
	}
	end := position + int(size)
	return opcode, true, script[position:end], end, true
}

// OPReturnText returns a human-readable UTF-8 payload from a standard
// OP_RETURN script. Binary protocol data and malformed push operations are
// intentionally ignored rather than shown as misleading text.
func OPReturnText(script []byte) (string, bool) {
	if len(script) < 2 || script[0] != 0x6a {
		return "", false
	}
	payload := make([]byte, 0, len(script)-1)
	for position := 1; position < len(script); {
		opcode := script[position]
		position++
		var size int
		switch {
		case opcode == 0:
			continue
		case opcode <= 75:
			size = int(opcode)
		case opcode == 0x4c:
			if position >= len(script) {
				return "", false
			}
			size = int(script[position])
			position++
		case opcode == 0x4d:
			if position+2 > len(script) {
				return "", false
			}
			size = int(binary.LittleEndian.Uint16(script[position : position+2]))
			position += 2
		default:
			return "", false
		}
		if size > len(script)-position || len(payload)+size > 1024 {
			return "", false
		}
		payload = append(payload, script[position:position+size]...)
		position += size
	}
	if !utf8.Valid(payload) {
		return "", false
	}
	message := strings.TrimSpace(string(payload))
	if message == "" {
		return "", false
	}
	for _, character := range message {
		if !unicode.IsPrint(character) && character != '\n' && character != '\r' && character != '\t' {
			return "", false
		}
	}
	return message, true
}

// ParseTransaction decodes the input references, outputs, and witness stack
// items needed to compute wallet effects and detect protocol markers.
func ParseTransaction(rawHex string) (Transaction, error) {
	raw, err := hex.DecodeString(rawHex)
	if err != nil {
		return Transaction{}, fmt.Errorf("decode transaction hex: %w", err)
	}
	r := txReader{data: raw}
	if _, err := r.take(4); err != nil {
		return Transaction{}, errors.New("transaction is missing its version")
	}
	segwit := len(r.data)-r.pos >= 2 && r.data[r.pos] == 0 && r.data[r.pos+1] != 0
	if segwit {
		r.pos += 2
	}
	inputCount, err := r.varInt()
	if err != nil {
		return Transaction{}, fmt.Errorf("read input count: %w", err)
	}
	if inputCount > 100_000 {
		return Transaction{}, errors.New("unreasonable input count")
	}
	tx := Transaction{Inputs: make([]TxInput, 0, inputCount)}
	for i := uint64(0); i < inputCount; i++ {
		previous, err := r.take(32)
		if err != nil {
			return Transaction{}, fmt.Errorf("read input %d transaction id: %w", i, err)
		}
		voutBytes, err := r.take(4)
		if err != nil {
			return Transaction{}, fmt.Errorf("read input %d output index: %w", i, err)
		}
		scriptLength, err := r.varInt()
		if err != nil {
			return Transaction{}, fmt.Errorf("read input %d script length: %w", i, err)
		}
		if _, err := r.takeSize(scriptLength); err != nil {
			return Transaction{}, fmt.Errorf("read input %d script: %w", i, err)
		}
		sequenceBytes, err := r.take(4)
		if err != nil {
			return Transaction{}, fmt.Errorf("read input %d sequence: %w", i, err)
		}
		reverse(previous)
		tx.Inputs = append(tx.Inputs, TxInput{PreviousTxID: hex.EncodeToString(previous), PreviousVout: binary.LittleEndian.Uint32(voutBytes), Sequence: binary.LittleEndian.Uint32(sequenceBytes)})
	}
	outputCount, err := r.varInt()
	if err != nil {
		return Transaction{}, fmt.Errorf("read output count: %w", err)
	}
	if outputCount > 100_000 {
		return Transaction{}, errors.New("unreasonable output count")
	}
	tx.Outputs = make([]TxOutput, 0, outputCount)
	for i := uint64(0); i < outputCount; i++ {
		valueBytes, err := r.take(8)
		if err != nil {
			return Transaction{}, fmt.Errorf("read output %d value: %w", i, err)
		}
		scriptLength, err := r.varInt()
		if err != nil {
			return Transaction{}, fmt.Errorf("read output %d script length: %w", i, err)
		}
		script, err := r.takeSize(scriptLength)
		if err != nil {
			return Transaction{}, fmt.Errorf("read output %d script: %w", i, err)
		}
		tx.Outputs = append(tx.Outputs, TxOutput{Value: binary.LittleEndian.Uint64(valueBytes), Script: append([]byte(nil), script...)})
	}
	if segwit {
		tx.Witnesses = make([][][]byte, inputCount)
		for i := uint64(0); i < inputCount; i++ {
			items, err := r.varInt()
			if err != nil {
				return Transaction{}, fmt.Errorf("read witness %d item count: %w", i, err)
			}
			tx.Witnesses[i] = make([][]byte, 0, items)
			for j := uint64(0); j < items; j++ {
				length, err := r.varInt()
				if err != nil {
					return Transaction{}, fmt.Errorf("read witness item length: %w", err)
				}
				item, err := r.takeSize(length)
				if err != nil {
					return Transaction{}, fmt.Errorf("read witness item: %w", err)
				}
				tx.Witnesses[i] = append(tx.Witnesses[i], append([]byte(nil), item...))
			}
		}
	}
	if _, err := r.take(4); err != nil {
		return Transaction{}, errors.New("transaction is missing its locktime")
	}
	if r.pos != len(r.data) {
		return Transaction{}, errors.New("transaction has trailing data")
	}
	return tx, nil
}

type txReader struct {
	data []byte
	pos  int
}

func (r *txReader) take(n int) ([]byte, error) {
	if n < 0 || n > len(r.data)-r.pos {
		return nil, errors.New("unexpected end of transaction")
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *txReader) takeSize(n uint64) ([]byte, error) {
	if n > uint64(len(r.data)-r.pos) {
		return nil, errors.New("unexpected end of transaction")
	}
	return r.take(int(n))
}

func (r *txReader) varInt() (uint64, error) {
	prefix, err := r.take(1)
	if err != nil {
		return 0, err
	}
	switch prefix[0] {
	case 0xfd:
		b, err := r.take(2)
		if err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint16(b)), nil
	case 0xfe:
		b, err := r.take(4)
		if err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint32(b)), nil
	case 0xff:
		b, err := r.take(8)
		if err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint64(b), nil
	default:
		return uint64(prefix[0]), nil
	}
}

func reverse(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}
