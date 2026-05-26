// Package ws implements the WebSocket protocol as defined in RFC 6455.
// https://datatracker.ietf.org/doc/html/rfc6455
package ws

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
)

// requiredHeaders は RFC 6455 Section 4.2.1 で定められた、
// サーバーが必ず検証しなければならないクライアントハンドシェイクヘッダー。
var requiredHeaders = []string{
	"Upgrade",
	"Connection",
	"Sec-WebSocket-Key",
	"Sec-WebSocket-Version",
}

// guid は RFC 6455 Section 1.3 で定義された固定のマジックGUID。
// Sec-WebSocket-Accept の計算に使用する。
// この値はプロトコルの一部として固定されており、変更してはならない。
const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Upgrade は通常のHTTPリクエストをWebSocket接続にアップグレードする。
//
// RFC 6455 Section 4.2 で定められたサーバーハンドシェイク手順:
//  1. リクエストがGETメソッドであることを確認 (Section 4.2.1 item 1)
//  2. 必須ヘッダーの存在を検証
//  3. Sec-WebSocket-Version が "13" であることを確認 (Section 4.2.1 item 6)
//  4. Sec-WebSocket-Accept を計算して 101 を返す (Section 4.2.2)
//  5. http.Hijacker でコネクションを奪取し net.Conn として返す
func Upgrade(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if r.Method != http.MethodGet {
		return nil, fmt.Errorf("ws: method must be GET, got %s", r.Method)
	}

	if err := validateHeaders(r); err != nil {
		return nil, err
	}

	// RFC 6455 Section 4.2.2 Step 5.4:
	// Sec-WebSocket-Accept = base64(SHA1(key + guid))
	key := r.Header.Get("Sec-WebSocket-Key")
	accept := computeAccept(key)

	// RFC 6455 Section 4.2.2:
	// 101 Switching Protocols と共に下記ヘッダーを返す。
	// Connection と Upgrade は大文字小文字を問わないが慣例に従う。
	w.Header().Set("Upgrade", "websocket")
	w.Header().Set("Connection", "Upgrade")
	w.Header().Set("Sec-WebSocket-Accept", accept)
	w.WriteHeader(http.StatusSwitchingProtocols)

	// http.Hijacker は net/http が提供するインターフェース。
	// Hijack() を呼ぶことでHTTPサーバーの管理から切り離し、
	// 生のTCPコネクションを直接操作できるようにする。
	// WriteHeader の後に呼ぶことで 101 レスポンスが確実に送出される。
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("ws: ResponseWriter does not support hijacking")
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("ws: hijack failed: %w", err)
	}

	return conn, nil
}

// validateHeaders は RFC 6455 Section 4.2.1 に基づいてリクエストヘッダーを検証する。
func validateHeaders(r *http.Request) error {
	// RFC 6455 Section 4.2.1 item 3: Upgrade ヘッダーは "websocket" を含まなければならない
	if r.Header.Get("Upgrade") != "websocket" {
		return fmt.Errorf("ws: missing or invalid Upgrade header")
	}

	// RFC 6455 Section 4.2.1 item 4: Connection ヘッダーは "Upgrade" を含まなければならない
	if r.Header.Get("Connection") != "Upgrade" {
		return fmt.Errorf("ws: missing or invalid Connection header")
	}

	// RFC 6455 Section 4.2.1 item 5:
	// Sec-WebSocket-Key は16バイトのランダム値をbase64エンコードしたもの。
	// サーバーはデコードして16バイトになることを確認すべきだが、
	// 今回はヘッダーの存在確認のみとする。
	if r.Header.Get("Sec-WebSocket-Key") == "" {
		return fmt.Errorf("ws: missing Sec-WebSocket-Key header")
	}

	// RFC 6455 Section 4.2.1 item 6: バージョンは必ず "13" でなければならない
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return fmt.Errorf("ws: unsupported Sec-WebSocket-Version: %s", r.Header.Get("Sec-WebSocket-Version"))
	}

	return nil
}

// computeAccept は RFC 6455 Section 4.2.2 Step 5.4 で定められた
// Sec-WebSocket-Accept ヘッダーの値を計算する。
//
//	accept = base64(SHA1(clientKey + guid))
func computeAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
