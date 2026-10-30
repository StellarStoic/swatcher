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

func TestUnavailableErrorClassification(t *testing.T) {
	if !IsUnavailable(unavailable(errors.New("connection refused"))) {
		t.Fatal("connection failure was not classified as unavailable")
	}
	if IsUnavailable(rpcError{code: -1, message: "bad request"}) {
		t.Fatal("script-specific RPC error was classified as service unavailable")
	}
}

func TestSnapshotTimeoutIsServiceUnavailable(t *testing.T) {
	server := newTestElectrumServer(t)
	server.stall("blockchain.scripthash.subscribe")
	client := &Client{Address: server.address()}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	_, _, err := client.SnapshotSince(ctx, "script-hash", nil)
	if !IsUnavailable(err) {
		t.Fatalf("timeout error = %v, want service unavailable", err)
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

func TestSnapshotSubscribesCachesAndRefreshesOnNotification(t *testing.T) {
	server := newTestElectrumServer(t)
	client := &Client{Address: server.address()}
	changes := client.Changes()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	first, err := client.Snapshot(ctx, "script-hash")
	if err != nil {
		t.Fatal(err)
	}
	if first.Balance.Confirmed != 42 {
		t.Fatalf("initial balance = %d, want 42", first.Balance.Confirmed)
	}

	second, err := client.Snapshot(ctx, "script-hash")
	if err != nil {
		t.Fatal(err)
	}
	if second.Balance.Confirmed != 42 {
		t.Fatalf("cached balance = %d, want 42", second.Balance.Confirmed)
	}
	if got := server.methodCount("blockchain.scripthash.subscribe"); got != 1 {
		t.Fatalf("subscribe calls = %d, want 1", got)
	}
	if got := server.methodCount("blockchain.scripthash.get_history"); got != 1 {
		t.Fatalf("history calls before a change = %d, want 1", got)
	}
	if got := server.methodCount("blockchain.scripthash.get_balance"); got != 1 {
		t.Fatalf("balance calls before a change = %d, want 1", got)
	}

	server.setState("changed", 84)
	server.notify(t, "script-hash", "changed")
	select {
	case <-changes:
	case <-ctx.Done():
		t.Fatal("subscription change was not reported")
	}

	third, err := client.Snapshot(ctx, "script-hash")
	if err != nil {
		t.Fatal(err)
	}
	if third.Balance.Confirmed != 84 {
		t.Fatalf("refreshed balance = %d, want 84", third.Balance.Confirmed)
	}
	if got := server.methodCount("blockchain.scripthash.get_history"); got != 2 {
		t.Fatalf("history calls after a change = %d, want 2", got)
	}
	if got := server.methodCount("blockchain.scripthash.get_balance"); got != 2 {
		t.Fatalf("balance calls after a change = %d, want 2", got)
	}
	if violations := server.unsubscribedQueries(); len(violations) != 0 {
		t.Fatalf("queries were sent before subscribing on their connection: %v", violations)
	}
}

func TestSnapshotSinceUsesPersistedStatusWithoutReadingHistoryOrBalance(t *testing.T) {
	server := newTestElectrumServer(t)
	client := &Client{Address: server.address()}
	changes := client.Changes()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	knownStatus := "initial"
	if !client.NeedsRefresh("script-hash") {
		t.Fatal("unknown script hash did not require its initial refresh")
	}
	snapshot, refreshed, err := client.SnapshotSince(ctx, "script-hash", &knownStatus)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed {
		t.Fatal("matching persisted status caused an unnecessary refresh")
	}
	if snapshot.Balance != (Balance{}) || len(snapshot.History) != 0 || snapshot.Status != "" {
		t.Fatalf("unchanged snapshot = %+v, want zero value", snapshot)
	}
	if got := server.methodCount("blockchain.scripthash.subscribe"); got != 1 {
		t.Fatalf("subscribe calls = %d, want 1", got)
	}
	if got := server.methodCount("blockchain.scripthash.get_history"); got != 0 {
		t.Fatalf("history calls = %d, want 0", got)
	}
	if got := server.methodCount("blockchain.scripthash.get_balance"); got != 0 {
		t.Fatalf("balance calls = %d, want 0", got)
	}
	if client.NeedsRefresh("script-hash") {
		t.Fatal("unchanged subscribed script hash still requires refresh")
	}

	server.setState("changed", 84)
	server.notify(t, "script-hash", "changed")
	select {
	case <-changes:
	case <-ctx.Done():
		t.Fatal("subscription change was not reported")
	}
	if got := client.TakeChanges(); len(got) != 1 || got[0] != "script-hash" {
		t.Fatalf("changed script hashes = %v, want [script-hash]", got)
	}
	if !client.NeedsRefresh("script-hash") {
		t.Fatal("dirty subscription was not marked for refresh")
	}

	snapshot, refreshed, err = client.SnapshotSince(ctx, "script-hash", &knownStatus)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("changed status did not refresh the snapshot")
	}
	if snapshot.Status != "changed" || snapshot.Balance.Confirmed != 84 {
		t.Fatalf("refreshed snapshot = %+v", snapshot)
	}
	if got := server.methodCount("blockchain.scripthash.get_history"); got != 1 {
		t.Fatalf("history calls after change = %d, want 1", got)
	}
	if got := server.methodCount("blockchain.scripthash.get_balance"); got != 1 {
		t.Fatalf("balance calls after change = %d, want 1", got)
	}
	if client.NeedsRefresh("script-hash") {
		t.Fatal("refreshed subscription remained dirty")
	}
}

func TestSnapshotSinceKeepsPersistedStateAcrossUnchangedReconnect(t *testing.T) {
	server := newTestElectrumServer(t)
	client := &Client{Address: server.address()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	knownStatus := "initial"

	if _, available, err := client.SnapshotSince(ctx, "script-hash", &knownStatus); err != nil {
		t.Fatal(err)
	} else if available {
		t.Fatal("matching persisted status loaded a snapshot")
	}
	server.disconnect(t)
	waitForDisconnectedSession(t, client)
	if !client.NeedsRefresh("script-hash") {
		t.Fatal("disconnected restored subscription was not marked for refresh")
	}

	snapshot, available, err := client.SnapshotSince(ctx, "script-hash", &knownStatus)
	if err != nil {
		t.Fatal(err)
	}
	if available || snapshot.Balance != (Balance{}) || len(snapshot.History) != 0 || snapshot.Status != "" {
		t.Fatalf("unchanged reconnect returned %+v, available=%v; want persisted application state", snapshot, available)
	}
	if got := server.methodCount("blockchain.scripthash.get_history"); got != 0 {
		t.Fatalf("history calls after unchanged reconnect = %d, want 0", got)
	}
	if got := server.methodCount("blockchain.scripthash.get_balance"); got != 0 {
		t.Fatalf("balance calls after unchanged reconnect = %d, want 0", got)
	}

	snapshot, available, err = client.SnapshotSince(ctx, "script-hash", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !available || snapshot.Balance.Confirmed != 42 {
		t.Fatalf("caller without persisted state received %+v, available=%v", snapshot, available)
	}
}

func TestChangesRetainDistinctHashesWhenWakeupsCoalesce(t *testing.T) {
	server := newTestElectrumServer(t)
	client := &Client{Address: server.address()}
	changes := client.Changes()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	knownStatus := "initial"

	for _, scriptHash := range []string{"hash-b", "hash-a"} {
		if _, refreshed, err := client.SnapshotSince(ctx, scriptHash, &knownStatus); err != nil {
			t.Fatal(err)
		} else if refreshed {
			t.Fatalf("%s unexpectedly refreshed", scriptHash)
		}
	}
	server.notify(t, "hash-b", "changed-b")
	server.notify(t, "hash-a", "changed-a")
	select {
	case <-changes:
	case <-ctx.Done():
		t.Fatal("subscription changes were not reported")
	}

	deadline := time.Now().Add(time.Second)
	seen := make(map[string]bool)
	for {
		for _, scriptHash := range client.TakeChanges() {
			seen[scriptHash] = true
		}
		if seen["hash-a"] && seen["hash-b"] {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("changed script hashes = %v, want both hashes", seen)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestSnapshotSinceRestoredWalletAvoidsHistoryAndBalanceSweep(t *testing.T) {
	server := newTestElectrumServer(t)
	client := &Client{Address: server.address()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	knownStatus := "initial"
	const watches = 2_048

	for index := 0; index < watches; index++ {
		scriptHash := fmt.Sprintf("hash-%d", index)
		if _, available, err := client.SnapshotSince(ctx, scriptHash, &knownStatus); err != nil {
			t.Fatal(err)
		} else if available {
			t.Fatalf("%s unnecessarily loaded history and balance", scriptHash)
		}
	}
	if got := server.methodCount("blockchain.scripthash.subscribe"); got != watches {
		t.Fatalf("subscribe calls = %d, want %d", got, watches)
	}
	if got := server.methodCount("blockchain.scripthash.get_history"); got != 0 {
		t.Fatalf("history calls = %d, want 0", got)
	}
	if got := server.methodCount("blockchain.scripthash.get_balance"); got != 0 {
		t.Fatalf("balance calls = %d, want 0", got)
	}
}

func TestSnapshotReconnectsAndResubscribes(t *testing.T) {
	server := newTestElectrumServer(t)
	client := &Client{Address: server.address()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := client.Snapshot(ctx, "script-hash"); err != nil {
		t.Fatal(err)
	}
	server.setState("after-reconnect", 99)
	server.disconnect(t)
	waitForDisconnectedSession(t, client)
	if !client.NeedsRefresh("script-hash") {
		t.Fatal("disconnected subscription was not marked for refresh")
	}

	snapshot, err := client.Snapshot(ctx, "script-hash")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Balance.Confirmed != 99 {
		t.Fatalf("balance after reconnect = %d, want 99", snapshot.Balance.Confirmed)
	}
	if got := server.methodCount("blockchain.scripthash.subscribe"); got != 2 {
		t.Fatalf("subscribe calls after reconnect = %d, want 2", got)
	}
	if violations := server.unsubscribedQueries(); len(violations) != 0 {
		t.Fatalf("queries were sent before resubscribing: %v", violations)
	}
}

func waitForDisconnectedSession(t *testing.T, client *Client) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		client.mu.Lock()
		closed := client.session == nil || client.session.closed()
		client.mu.Unlock()
		if closed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("client did not observe the disconnected session")
		}
		time.Sleep(time.Millisecond)
	}
}

type testElectrumServer struct {
	listener net.Listener

	mu          sync.Mutex
	connections []net.Conn
	current     net.Conn
	methods     map[string]int
	violations  []string
	status      string
	balance     int64
	stallMethod string

	writeMu sync.Mutex
}

func newTestElectrumServer(t *testing.T) *testElectrumServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &testElectrumServer{
		listener: listener,
		methods:  make(map[string]int),
		status:   "initial",
		balance:  42,
	}
	go server.acceptLoop()
	t.Cleanup(func() {
		_ = listener.Close()
		server.mu.Lock()
		connections := append([]net.Conn(nil), server.connections...)
		server.mu.Unlock()
		for _, conn := range connections {
			_ = conn.Close()
		}
	})
	return server
}

func (s *testElectrumServer) address() string {
	return s.listener.Addr().String()
}

func (s *testElectrumServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.connections = append(s.connections, conn)
		s.current = conn
		s.mu.Unlock()
		go s.serve(conn)
	}
}

func (s *testElectrumServer) serve(conn net.Conn) {
	subscribed := make(map[string]bool)
	decoder := json.NewDecoder(bufio.NewReader(conn))
	for {
		var request struct {
			ID     uint64            `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if decoder.Decode(&request) != nil {
			return
		}
		var scriptHash string
		if len(request.Params) > 0 {
			_ = json.Unmarshal(request.Params[0], &scriptHash)
		}

		s.mu.Lock()
		s.methods[request.Method]++
		status, balance, stallMethod := s.status, s.balance, s.stallMethod
		if (request.Method == "blockchain.scripthash.get_history" || request.Method == "blockchain.scripthash.get_balance") && !subscribed[scriptHash] {
			s.violations = append(s.violations, request.Method)
		}
		s.mu.Unlock()
		if request.Method == stallMethod {
			continue
		}

		var result any
		switch request.Method {
		case "blockchain.scripthash.subscribe":
			subscribed[scriptHash] = true
			result = status
		case "blockchain.scripthash.get_history":
			result = []HistoryItem{{TxHash: "tx", Height: 1}}
		case "blockchain.scripthash.get_balance":
			result = Balance{Confirmed: balance}
		default:
			result = nil
		}
		s.writeMu.Lock()
		err := json.NewEncoder(conn).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
		s.writeMu.Unlock()
		if err != nil {
			return
		}
	}
}

func (s *testElectrumServer) setState(status string, balance int64) {
	s.mu.Lock()
	s.status = status
	s.balance = balance
	s.mu.Unlock()
}

func (s *testElectrumServer) stall(method string) {
	s.mu.Lock()
	s.stallMethod = method
	s.mu.Unlock()
}

func (s *testElectrumServer) notify(t *testing.T, scriptHash, status string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		conn := s.current
		s.mu.Unlock()
		if conn != nil {
			s.writeMu.Lock()
			err := json.NewEncoder(conn).Encode(map[string]any{
				"jsonrpc": "2.0",
				"method":  "blockchain.scripthash.subscribe",
				"params":  []any{scriptHash, status},
			})
			s.writeMu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("test server did not receive a connection")
		}
		time.Sleep(time.Millisecond)
	}
}

func (s *testElectrumServer) disconnect(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	conn := s.current
	s.mu.Unlock()
	if conn == nil {
		t.Fatal("test server has no connection to close")
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}

func (s *testElectrumServer) methodCount(method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.methods[method]
}

func (s *testElectrumServer) unsubscribedQueries() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.violations...)
}
