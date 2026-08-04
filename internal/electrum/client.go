// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package electrum

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/s-watcher/s-watcher/internal/bitcoin"
)

type Client struct {
	Address string
	nextID  atomic.Uint64
}

type HistoryItem struct {
	TxHash string `json:"tx_hash"`
	Height int64  `json:"height"`
	Fee    int64  `json:"fee,omitempty"`
}

type Balance struct {
	Confirmed   int64 `json:"confirmed"`
	Unconfirmed int64 `json:"unconfirmed"`
}

type Snapshot struct {
	Balance Balance
	History []HistoryItem
}

type Effect struct {
	Received uint64
	Sent     uint64
	OPReturn []string
}

type response struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Snapshot(ctx context.Context, scriptHash string) (Snapshot, error) {
	var history []HistoryItem
	if err := c.call(ctx, "blockchain.scripthash.get_history", []any{scriptHash}, &history); err != nil {
		return Snapshot{}, err
	}
	var balance Balance
	if err := c.call(ctx, "blockchain.scripthash.get_balance", []any{scriptHash}, &balance); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Balance: balance, History: history}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	var result []any
	return c.call(ctx, "server.version", []any{"s-watcher", "1.4"}, &result)
}

// TransactionEffect calculates exactly how many satoshis entered and left a
// set of wallet scripts in one transaction. Previous transactions are fetched
// from the same local Electrs instance to resolve input values.
func (c *Client) TransactionEffect(ctx context.Context, txID string, scripts [][]byte) (Effect, error) {
	tx, err := c.transaction(ctx, txID)
	if err != nil {
		return Effect{}, err
	}
	var effect Effect
	watched := make(map[string]bool, len(scripts))
	for _, script := range scripts {
		watched[string(script)] = true
	}
	for _, output := range tx.Outputs {
		if message, ok := bitcoin.OPReturnText(output.Script); ok {
			effect.OPReturn = append(effect.OPReturn, message)
		}
		if watched[string(output.Script)] {
			effect.Received += output.Value
		}
	}
	for _, input := range tx.Inputs {
		if input.PreviousTxID == "0000000000000000000000000000000000000000000000000000000000000000" {
			continue
		}
		previous, err := c.transaction(ctx, input.PreviousTxID)
		if err != nil {
			return Effect{}, fmt.Errorf("fetch previous transaction %s: %w", input.PreviousTxID, err)
		}
		if uint64(input.PreviousVout) >= uint64(len(previous.Outputs)) {
			return Effect{}, fmt.Errorf("previous output %s:%d does not exist", input.PreviousTxID, input.PreviousVout)
		}
		output := previous.Outputs[input.PreviousVout]
		if watched[string(output.Script)] {
			effect.Sent += output.Value
		}
	}
	return effect, nil
}

func (c *Client) transaction(ctx context.Context, txID string) (bitcoin.Transaction, error) {
	var raw string
	if err := c.call(ctx, "blockchain.transaction.get", []any{txID, false}, &raw); err != nil {
		return bitcoin.Transaction{}, fmt.Errorf("get transaction %s: %w", txID, err)
	}
	tx, err := bitcoin.ParseTransaction(raw)
	if err != nil {
		return bitcoin.Transaction{}, fmt.Errorf("parse transaction %s: %w", txID, err)
	}
	return tx, nil
}

func (c *Client) call(ctx context.Context, method string, params []any, out any) error {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.Address)
	if err != nil {
		return fmt.Errorf("connect to electrs: %w", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	id := c.nextID.Add(1)
	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("write electrum request: %w", err)
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("read electrum response: %w", err)
	}
	var res response
	if err := json.Unmarshal(line, &res); err != nil {
		return fmt.Errorf("decode electrum response: %w", err)
	}
	if res.ID != id {
		return errors.New("electrum returned an unexpected response id")
	}
	if res.Error != nil {
		return fmt.Errorf("electrum error %d: %s", res.Error.Code, res.Error.Message)
	}
	if err := json.Unmarshal(res.Result, out); err != nil {
		return fmt.Errorf("decode electrum result: %w", err)
	}
	return nil
}
