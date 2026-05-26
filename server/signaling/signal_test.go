package signaling

import (
	"encoding/json"
	"testing"

	"github.com/samber/mo"
)

// TestParseSignal_Valid は有効なシグナルJSONが正しくパースされることを検証する。
func TestParseSignal_Valid(t *testing.T) {
	data := `{"type":"offer","payload":{"sdp":"v=0..."}}`
	result := parseSignal([]byte(data))
	sig, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig.Type != SignalOffer {
		t.Errorf("type = %q, want %q", sig.Type, SignalOffer)
	}
}

// TestParseSignal_InvalidJSON は不正なJSONがエラーになることを検証する。
func TestParseSignal_InvalidJSON(t *testing.T) {
	result := parseSignal([]byte(`{invalid`))
	if result.IsOk() {
		t.Error("expected error for invalid JSON, got ok")
	}
}

// TestValidateClientSignal_Allowed は許可種別 (offer/answer/candidate) が通ることを検証する。
func TestValidateClientSignal_Allowed(t *testing.T) {
	allowed := []SignalType{SignalOffer, SignalAnswer, SignalCandidate}
	for _, typ := range allowed {
		result := validateClientSignal(Signal{Type: typ})
		if _, err := result.Get(); err != nil {
			t.Errorf("type %q should be allowed, got error: %v", typ, err)
		}
	}
}

// TestValidateClientSignal_Forbidden は ready/bye がクライアントから送られた場合に
// エラーになることを検証する。
// これらはサーバーが生成するメタシグナルであり、クライアントが偽装できてはならない。
func TestValidateClientSignal_Forbidden(t *testing.T) {
	forbidden := []SignalType{SignalReady, SignalBye, "unknown"}
	for _, typ := range forbidden {
		result := validateClientSignal(Signal{Type: typ})
		if result.IsOk() {
			t.Errorf("type %q should be forbidden, got ok", typ)
		}
	}
}

// TestRelay_Success は有効なシグナルが相手ピアの send チャネルに届くことを検証する。
func TestRelay_Success(t *testing.T) {
	peer := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}

	raw, _ := json.Marshal(Signal{Type: SignalOffer})
	result := relay(raw, mo.Some(peer))
	if _, err := result.Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case got := <-peer.send:
		var sig Signal
		if err := json.Unmarshal(got, &sig); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if sig.Type != SignalOffer {
			t.Errorf("relayed type = %q, want offer", sig.Type)
		}
	default:
		t.Error("expected message in send channel, got nothing")
	}
}

// TestRelay_NoPeer は相手ピアが存在しない場合にエラーになることを検証する。
// P2P 接続前に誤ってシグナルを送ってきたケース。
func TestRelay_NoPeer(t *testing.T) {
	raw, _ := json.Marshal(Signal{Type: SignalCandidate})
	result := relay(raw, mo.None[*Peer]())
	if result.IsOk() {
		t.Error("expected error when no peer, got ok")
	}
}

// TestRelay_ForbiddenType はクライアントが ready を送ってきた場合に中継しないことを検証する。
func TestRelay_ForbiddenType(t *testing.T) {
	peer := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}

	raw, _ := json.Marshal(Signal{Type: SignalReady})
	result := relay(raw, mo.Some(peer))
	if result.IsOk() {
		t.Error("expected error for forbidden type, got ok")
	}

	// send チャネルには何も届いていないはず
	select {
	case <-peer.send:
		t.Error("ready signal should not be relayed")
	default:
	}
}

// TestReadyByeSignals は起動時に生成される readySignal / byeSignal が
// 正しい JSON であることを検証する。
func TestReadyByeSignals(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want SignalType
	}{
		{"ready", readySignal, SignalReady},
		{"bye", byeSignal, SignalBye},
	} {
		var sig Signal
		if err := json.Unmarshal(tc.data, &sig); err != nil {
			t.Errorf("%s: unmarshal error: %v", tc.name, err)
		}
		if sig.Type != tc.want {
			t.Errorf("%s: type = %q, want %q", tc.name, sig.Type, tc.want)
		}
	}
}
