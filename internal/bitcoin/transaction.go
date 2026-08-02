package bitcoin

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

type Transaction struct {
	Inputs  []TxInput
	Outputs []TxOutput
}

type TxInput struct {
	PreviousTxID string
	PreviousVout uint32
}

type TxOutput struct {
	Value  uint64
	Script []byte
}

// ParseTransaction decodes the input references and outputs needed to compute
// the effect of a transaction on a watched script. Signatures and witnesses
// are intentionally skipped.
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
		if _, err := r.take(4); err != nil {
			return Transaction{}, fmt.Errorf("read input %d sequence: %w", i, err)
		}
		reverse(previous)
		tx.Inputs = append(tx.Inputs, TxInput{PreviousTxID: hex.EncodeToString(previous), PreviousVout: binary.LittleEndian.Uint32(voutBytes)})
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
		for i := uint64(0); i < inputCount; i++ {
			items, err := r.varInt()
			if err != nil {
				return Transaction{}, fmt.Errorf("read witness %d item count: %w", i, err)
			}
			for j := uint64(0); j < items; j++ {
				length, err := r.varInt()
				if err != nil {
					return Transaction{}, fmt.Errorf("read witness item length: %w", err)
				}
				if _, err := r.takeSize(length); err != nil {
					return Transaction{}, fmt.Errorf("read witness item: %w", err)
				}
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
