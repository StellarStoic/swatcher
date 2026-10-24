// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package electrum

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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

	deadline := time.Now().Add(time.Second)
	for {
		client.mu.Lock()
		closed := client.session == nil || client.session.closed()
		client.mu.Unlock()
		if closed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client did not observe the disconnected session")
		}
		time.Sleep(time.Millisecond)
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

type testElectrumServer struct {
	listener net.Listener

	mu          sync.Mutex
	connections []net.Conn
	current     net.Conn
	methods     map[string]int
	violations  []string
	status      string
	balance     int64

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
		status, balance := s.status, s.balance
		if (request.Method == "blockchain.scripthash.get_history" || request.Method == "blockchain.scripthash.get_balance") && !subscribed[scriptHash] {
			s.violations = append(s.violations, request.Method)
		}
		s.mu.Unlock()

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
