package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestComputeAccept は RFC 6455 Section 4.2.2 の計算例を検証する。
//
// RFC 6455 Section 1.3 にある公式サンプル:
//
//	Key:    "dGhlIHNhbXBsZSBub25jZQ=="
//	Accept: "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
func TestComputeAccept(t *testing.T) {
	got := computeAccept("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Errorf("computeAccept = %q, want %q", got, want)
	}
}

// TestUpgrade_Success は正常なWebSocketアップグレードリクエストを検証する。
//
// RFC 6455 Section 4.2.2: サーバーは 101 Switching Protocols と
// Sec-WebSocket-Accept を返さなければならない。
func TestUpgrade_Success(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	w := httptest.NewRecorder()
	_, err := Upgrade(w, req)

	// httptest.ResponseRecorder は http.Hijacker を実装しないため
	// Hijack でエラーになるが、それ以前の 101 レスポンス生成は検証できる。
	if w.Code != http.StatusSwitchingProtocols {
		t.Errorf("status = %d, want 101", w.Code)
	}
	if w.Header().Get("Sec-WebSocket-Accept") != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Errorf("Sec-WebSocket-Accept = %q, want correct value", w.Header().Get("Sec-WebSocket-Accept"))
	}
	_ = err // Hijack 失敗は想定内
}

// TestUpgrade_NonGetMethod は RFC 6455 Section 4.2.1 item 1 を検証する。
// GET 以外のメソッドはアップグレードを拒否しなければならない。
func TestUpgrade_NonGetMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ws", nil)
	w := httptest.NewRecorder()
	_, err := Upgrade(w, req)
	if err == nil {
		t.Error("expected error for non-GET method, got nil")
	}
}

// TestUpgrade_MissingUpgradeHeader は RFC 6455 Section 4.2.1 item 3 を検証する。
func TestUpgrade_MissingUpgradeHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	w := httptest.NewRecorder()
	_, err := Upgrade(w, req)
	if err == nil {
		t.Error("expected error for missing Upgrade header, got nil")
	}
}

// TestUpgrade_MissingConnectionHeader は RFC 6455 Section 4.2.1 item 4 を検証する。
func TestUpgrade_MissingConnectionHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	w := httptest.NewRecorder()
	_, err := Upgrade(w, req)
	if err == nil {
		t.Error("expected error for missing Connection header, got nil")
	}
}

// TestUpgrade_MissingKey は RFC 6455 Section 4.2.1 item 5 を検証する。
func TestUpgrade_MissingKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")

	w := httptest.NewRecorder()
	_, err := Upgrade(w, req)
	if err == nil {
		t.Error("expected error for missing Sec-WebSocket-Key, got nil")
	}
}

// TestUpgrade_WrongVersion は RFC 6455 Section 4.2.1 item 6 を検証する。
// バージョン 13 以外は拒否しなければならない。
func TestUpgrade_WrongVersion(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "8")

	w := httptest.NewRecorder()
	_, err := Upgrade(w, req)
	if err == nil {
		t.Error("expected error for unsupported version, got nil")
	}
}
