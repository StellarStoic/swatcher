// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package electrum

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/s-watcher/s-watcher/internal/bitcoin"
)

type Client struct {
	Address string
	nextID  atomic.Uint64

	mu      sync.Mutex
	session *session

	subscriptionsMu sync.Mutex
	subscriptions   map[string]*subscription
	changes         chan struct{}
}

type subscription struct {
	fetchMu sync.Mutex
	mu      sync.Mutex

	session     *session
	status      string
	generation  uint64
	dirty       bool
	initialized bool
	snapshot    Snapshot
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

type Header struct {
	Height int64 `json:"height"`
}

type Effect struct {
	Received                  uint64
	Sent                      uint64
	OPReturn                  []string
	Replaceable               bool
	Runestone                 bool
	Inscriptions              int
	ReceivedScripts           []string
	SpentScripts              []string
	SourceAddresses           []string
	DestinationAddresses      []string
	SpentScriptAmounts        map[string]uint64
	SourceAddressAmounts      map[string]uint64
	DestinationAddressAmounts map[string]uint64
}

type response struct {
	ID     *uint64           `json:"id,omitempty"`
	Method string            `json:"method,omitempty"`
	Params []json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage   `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type rpcError struct {
	code    int
	message string
}

func (e rpcError) Error() string {
	return fmt.Sprintf("electrum error %d: %s", e.code, e.message)
}

type callResult struct {
	result json.RawMessage
	err    error
}

type session struct {
	conn net.Conn

	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[uint64]chan callResult

	done      chan struct{}
	closeOnce sync.Once
	onNotify  func(*session, response)
}

func (c *Client) Snapshot(ctx context.Context, scriptHash string) (Snapshot, error) {
	sub := c.subscription(scriptHash)
	sub.fetchMu.Lock()
	defer sub.fetchMu.Unlock()

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		s, err := c.getSession(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		if err := c.ensureSubscribed(ctx, s, scriptHash, sub); err != nil {
			lastErr = err
			if !retryable(err) {
				return Snapshot{}, err
			}
			c.invalidate(s, err)
			continue
		}

		sub.mu.Lock()
		if sub.initialized && !sub.dirty {
			snapshot := cloneSnapshot(sub.snapshot)
			sub.mu.Unlock()
			return snapshot, nil
		}
		generation := sub.generation
		sub.mu.Unlock()

		var history []HistoryItem
		if err := s.call(ctx, c.nextID.Add(1), "blockchain.scripthash.get_history", []any{scriptHash}, &history); err != nil {
			lastErr = err
			if !retryable(err) {
				return Snapshot{}, err
			}
			c.invalidate(s, err)
			continue
		}
		var balance Balance
		if err := s.call(ctx, c.nextID.Add(1), "blockchain.scripthash.get_balance", []any{scriptHash}, &balance); err != nil {
			lastErr = err
			if !retryable(err) {
				return Snapshot{}, err
			}
			c.invalidate(s, err)
			continue
		}

		snapshot := Snapshot{Balance: balance, History: history}
		sub.mu.Lock()
		sub.snapshot = cloneSnapshot(snapshot)
		sub.initialized = true
		sub.dirty = sub.generation != generation
		sub.mu.Unlock()
		return snapshot, nil
	}
	return Snapshot{}, lastErr
}

// Changes reports script-hash subscription updates. The channel is buffered
// and deliberately coalesces bursts so callers can refresh all dirty watches
// once instead of starting one scan per Electrum notification.
func (c *Client) Changes() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.changes == nil {
		c.changes = make(chan struct{}, 1)
	}
	return c.changes
}

func (c *Client) Ping(ctx context.Context) error {
	var result []any
	return c.call(ctx, "server.version", []any{"s-watcher", "1.4"}, &result)
}

func (c *Client) TipHeight(ctx context.Context) (int64, error) {
	var header Header
	if err := c.call(ctx, "blockchain.headers.subscribe", []any{}, &header); err != nil {
		return 0, err
	}
	return header.Height, nil
}

func (c *Client) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	var raw string
	if err := c.call(ctx, "blockchain.block.header", []any{height}, &raw); err != nil {
		return time.Time{}, err
	}
	return blockTimeFromHeader(raw)
}

func blockTimeFromHeader(raw string) (time.Time, error) {
	header, err := hex.DecodeString(raw)
	if err != nil || len(header) != 80 {
		return time.Time{}, errors.New("electrs returned an invalid block header")
	}
	timestamp := binary.LittleEndian.Uint32(header[68:72])
	return time.Unix(int64(timestamp), 0).UTC(), nil
}

// TransactionEffect calculates exactly how many satoshis entered and left a
// set of wallet scripts in one transaction. Previous transactions are fetched
// from the same local Electrs instance to resolve input values.
func (c *Client) TransactionEffect(ctx context.Context, txID string, scripts [][]byte) (Effect, error) {
	tx, err := c.transaction(ctx, txID)
	if err != nil {
		return Effect{}, err
	}
	effect := Effect{
		Replaceable:  tx.SignalsRBF(),
		Runestone:    tx.HasRunestone(),
		Inscriptions: tx.InscriptionEnvelopeCount(),
	}
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
			effect.ReceivedScripts = append(effect.ReceivedScripts, string(output.Script))
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
		addInputEffect(&effect, previous.Outputs[input.PreviousVout], watched)
	}
	if effect.Sent > 0 {
		for _, output := range tx.Outputs {
			if watched[string(output.Script)] {
				continue
			}
			if address, ok := bitcoin.AddressFromScript(output.Script); ok {
				effect.DestinationAddresses = appendUnique(effect.DestinationAddresses, address)
				addAmount(&effect.DestinationAddressAmounts, address, output.Value)
			}
		}
	}
	return effect, nil
}

func addInputEffect(effect *Effect, output bitcoin.TxOutput, watched map[string]bool) {
	if watched[string(output.Script)] {
		effect.Sent += output.Value
		effect.SpentScripts = appendUnique(effect.SpentScripts, string(output.Script))
		addAmount(&effect.SpentScriptAmounts, string(output.Script), output.Value)
		return
	}
	if address, ok := bitcoin.AddressFromScript(output.Script); ok {
		effect.SourceAddresses = appendUnique(effect.SourceAddresses, address)
		addAmount(&effect.SourceAddressAmounts, address, output.Value)
	}
}

func addAmount(amounts *map[string]uint64, key string, value uint64) {
	if *amounts == nil {
		*amounts = make(map[string]uint64)
	}
	(*amounts)[key] += value
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
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
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		s, err := c.getSession(ctx)
		if err != nil {
			return err
		}
		err = s.call(ctx, c.nextID.Add(1), method, params, out)
		if err == nil || !retryable(err) {
			return err
		}
		lastErr = err
		c.invalidate(s, err)
	}
	return lastErr
}

func (c *Client) getSession(ctx context.Context) (*session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil && !c.session.closed() {
		return c.session, nil
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.Address)
	if err != nil {
		return nil, fmt.Errorf("connect to electrs: %w", err)
	}
	s := &session{
		conn:     conn,
		pending:  make(map[uint64]chan callResult),
		done:     make(chan struct{}),
		onNotify: c.handleNotification,
	}
	c.session = s
	go s.readLoop()
	return s, nil
}

func (c *Client) invalidate(s *session, err error) {
	c.mu.Lock()
	if c.session == s {
		c.session = nil
	}
	c.mu.Unlock()
	s.close(err)
}

func (c *Client) subscription(scriptHash string) *subscription {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()
	if c.subscriptions == nil {
		c.subscriptions = make(map[string]*subscription)
	}
	if c.subscriptions[scriptHash] == nil {
		c.subscriptions[scriptHash] = &subscription{dirty: true}
	}
	return c.subscriptions[scriptHash]
}

func (c *Client) ensureSubscribed(ctx context.Context, s *session, scriptHash string, sub *subscription) error {
	sub.mu.Lock()
	alreadySubscribed := sub.session == s
	sub.mu.Unlock()
	if alreadySubscribed {
		return nil
	}

	var status *string
	if err := s.call(ctx, c.nextID.Add(1), "blockchain.scripthash.subscribe", []any{scriptHash}, &status); err != nil {
		return err
	}
	statusValue := ""
	if status != nil {
		statusValue = *status
	}
	sub.mu.Lock()
	if sub.session != s || sub.status != statusValue {
		sub.generation++
		if sub.initialized && sub.status != statusValue {
			sub.dirty = true
		}
	}
	sub.session = s
	sub.status = statusValue
	sub.mu.Unlock()
	return nil
}

func (c *Client) handleNotification(s *session, message response) {
	if message.Method != "blockchain.scripthash.subscribe" || len(message.Params) < 2 {
		return
	}
	var scriptHash string
	var status *string
	if json.Unmarshal(message.Params[0], &scriptHash) != nil || json.Unmarshal(message.Params[1], &status) != nil || scriptHash == "" {
		return
	}
	statusValue := ""
	if status != nil {
		statusValue = *status
	}
	sub := c.subscription(scriptHash)
	sub.mu.Lock()
	if sub.session != s || sub.status == statusValue {
		sub.mu.Unlock()
		return
	}
	sub.status = statusValue
	sub.generation++
	sub.dirty = true
	sub.mu.Unlock()

	c.mu.Lock()
	changes := c.changes
	c.mu.Unlock()
	if changes != nil {
		select {
		case changes <- struct{}{}:
		default:
		}
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	clone := snapshot
	clone.History = append([]HistoryItem(nil), snapshot.History...)
	return clone
}

func retryable(err error) bool {
	var rpcErr rpcError
	return err != nil && !errors.As(err, &rpcErr) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func (s *session) call(ctx context.Context, id uint64, method string, params []any, out any) error {
	result := make(chan callResult, 1)
	s.pendingMu.Lock()
	if s.closed() {
		s.pendingMu.Unlock()
		return errors.New("electrum connection is closed")
	}
	s.pending[id] = result
	s.pendingMu.Unlock()

	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	s.writeMu.Lock()
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = s.conn.SetWriteDeadline(deadline)
	err := json.NewEncoder(s.conn).Encode(req)
	_ = s.conn.SetWriteDeadline(time.Time{})
	s.writeMu.Unlock()
	if err != nil {
		s.removePending(id)
		s.close(fmt.Errorf("write electrum request: %w", err))
		return fmt.Errorf("write electrum request: %w", err)
	}

	select {
	case reply := <-result:
		if reply.err != nil {
			return reply.err
		}
		if err := json.Unmarshal(reply.result, out); err != nil {
			return fmt.Errorf("decode electrum result: %w", err)
		}
		return nil
	case <-ctx.Done():
		s.removePending(id)
		return ctx.Err()
	case <-s.done:
		s.removePending(id)
		return errors.New("electrum connection closed")
	}
}

func (s *session) readLoop() {
	decoder := json.NewDecoder(bufio.NewReader(s.conn))
	for {
		var message response
		if err := decoder.Decode(&message); err != nil {
			s.close(fmt.Errorf("read electrum response: %w", err))
			return
		}
		if message.ID == nil {
			if s.onNotify != nil && message.Method != "" {
				s.onNotify(s, message)
			}
			continue
		}
		s.pendingMu.Lock()
		pending := s.pending[*message.ID]
		delete(s.pending, *message.ID)
		s.pendingMu.Unlock()
		if pending == nil {
			continue
		}
		if message.Error != nil {
			pending <- callResult{err: rpcError{code: message.Error.Code, message: message.Error.Message}}
		} else {
			pending <- callResult{result: message.Result}
		}
	}
}

func (s *session) removePending(id uint64) {
	s.pendingMu.Lock()
	delete(s.pending, id)
	s.pendingMu.Unlock()
}

func (s *session) close(err error) {
	if err == nil {
		err = errors.New("electrum connection closed")
	}
	s.closeOnce.Do(func() {
		_ = s.conn.Close()
		close(s.done)
		s.pendingMu.Lock()
		pending := s.pending
		s.pending = make(map[uint64]chan callResult)
		s.pendingMu.Unlock()
		for _, waiter := range pending {
			waiter <- callResult{err: err}
		}
	})
}

func (s *session) closed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}
