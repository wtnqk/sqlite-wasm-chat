package ws

import (
	"bytes"
	"testing"
)

// --- ReadFrame テスト ---

// TestReadFrame_ShortTextFrame は RFC 6455 Section 5.7 の Example 1 を検証する。
//
// "Hello" をテキストフレームとして送るクライアント→サーバーのフレーム:
//
//	0x81 0x85  (FIN=1, Text, MASK=1, len=5)
//	0x37 0xfa 0x21 0x3d  (masking key)
//	0x7f 0x9f 0x4d 0x51 0x58  (masked "Hello")
func TestReadFrame_ShortTextFrame(t *testing.T) {
	// RFC 6455 Section 5.7 Example 1 の実データ
	data := []byte{
		0x81, 0x85,
		0x37, 0xfa, 0x21, 0x3d,
		0x7f, 0x9f, 0x4d, 0x51, 0x58,
	}

	result := ReadFrame(bytes.NewReader(data))
	frame, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if frame.Opcode != OpcodeText {
		t.Errorf("opcode = %v, want OpcodeText", frame.Opcode)
	}
	if !frame.FIN {
		t.Error("FIN should be true")
	}
	if string(frame.Payload) != "Hello" {
		t.Errorf("payload = %q, want %q", frame.Payload, "Hello")
	}
}

// TestReadFrame_UnmaskedTextFrame は RFC 6455 Section 5.7 の Example 2 を検証する。
//
// サーバー→クライアントのフレームはマスクなし:
//
//	0x81 0x05 0x48 0x65 0x6c 0x6c 0x6f  ("Hello")
func TestReadFrame_UnmaskedTextFrame(t *testing.T) {
	data := []byte{0x81, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f}

	result := ReadFrame(bytes.NewReader(data))
	frame, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(frame.Payload) != "Hello" {
		t.Errorf("payload = %q, want %q", frame.Payload, "Hello")
	}
}

// TestReadFrame_ExtendedLen16 は RFC 6455 Section 5.2 の
// 16ビット拡張ペイロード長 (126〜65535バイト) を検証する。
func TestReadFrame_ExtendedLen16(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 200)

	// フレーム構築: FIN=1, Text, MASK=0, len=126, extended=200
	var buf bytes.Buffer
	buf.Write([]byte{0x81, 126, 0x00, 200})
	buf.Write(payload)

	result := ReadFrame(&buf)
	frame, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(frame.Payload) != 200 {
		t.Errorf("payload length = %d, want 200", len(frame.Payload))
	}
}

// TestReadFrame_ExtendedLen64 は RFC 6455 Section 5.2 の
// 64ビット拡張ペイロード長 (65536バイト以上) を検証する。
func TestReadFrame_ExtendedLen64(t *testing.T) {
	payload := bytes.Repeat([]byte("b"), 70000)

	var buf bytes.Buffer
	// FIN=1, Text, MASK=0, len=127, extended=70000 (8バイト big-endian)
	buf.Write([]byte{0x81, 127,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x11, 0x70,
	})
	buf.Write(payload)

	result := ReadFrame(&buf)
	frame, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(frame.Payload) != 70000 {
		t.Errorf("payload length = %d, want 70000", len(frame.Payload))
	}
}

// TestReadFrame_PingFrame は RFC 6455 Section 5.5.2 の Ping フレームを検証する。
//
// Ping フレームは Opcode=0x9 で、ペイロードは 125 バイト以下でなければならない。
func TestReadFrame_PingFrame(t *testing.T) {
	// FIN=1, Ping, MASK=0, len=0
	data := []byte{0x89, 0x00}

	result := ReadFrame(bytes.NewReader(data))
	frame, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if frame.Opcode != OpcodePing {
		t.Errorf("opcode = %v, want OpcodePing", frame.Opcode)
	}
}

// TestReadFrame_CloseFrame は RFC 6455 Section 5.5.1 の Close フレームを検証する。
//
// Close フレームはステータスコード (2バイト big-endian) を含む場合がある。
// ステータスコード 1000 は "Normal Closure" を意味する (Section 7.4.1)。
func TestReadFrame_CloseFrame(t *testing.T) {
	// FIN=1, Close, MASK=0, len=2, status=1000
	data := []byte{0x88, 0x02, 0x03, 0xe8}

	result := ReadFrame(bytes.NewReader(data))
	frame, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if frame.Opcode != OpcodeClose {
		t.Errorf("opcode = %v, want OpcodeClose", frame.Opcode)
	}
	if len(frame.Payload) != 2 {
		t.Errorf("close payload length = %d, want 2 (status code)", len(frame.Payload))
	}
}

// TestReadFrame_EmptyPayload はペイロードが空のフレームを検証する。
func TestReadFrame_EmptyPayload(t *testing.T) {
	data := []byte{0x81, 0x00}

	result := ReadFrame(bytes.NewReader(data))
	frame, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(frame.Payload) != 0 {
		t.Errorf("payload length = %d, want 0", len(frame.Payload))
	}
}

// TestReadFrame_Truncated は不完全なフレームがエラーになることを検証する。
// ネットワーク断など途中でデータが途切れたケース。
func TestReadFrame_Truncated(t *testing.T) {
	// ヘッダーは len=5 と言っているがペイロードが足りない
	data := []byte{0x81, 0x05, 0x48, 0x65}

	result := ReadFrame(bytes.NewReader(data))
	_, err := result.Get()
	if err == nil {
		t.Error("expected error for truncated frame, got nil")
	}
}

// TestReadFrame_EmptyReader は空のリーダーがエラーになることを検証する。
func TestReadFrame_EmptyReader(t *testing.T) {
	result := ReadFrame(bytes.NewReader(nil))
	_, err := result.Get()
	if err == nil {
		t.Error("expected error for empty reader, got nil")
	}
}

// --- WriteFrame テスト ---

// TestWriteFrame_RoundTrip は WriteFrame → ReadFrame のラウンドトリップを検証する。
//
// RFC 6455 Section 5.1:
// サーバーからのフレームはマスクなしで送信される。
// WriteFrame で書いたものを ReadFrame で正しく読み返せることを確認する。
func TestWriteFrame_RoundTrip(t *testing.T) {
	original := Frame{
		FIN:     true,
		Opcode:  OpcodeText,
		Payload: []byte("Hello, World!"),
	}

	var buf bytes.Buffer
	result := WriteFrame(&buf, original)
	if _, err := result.Get(); err != nil {
		t.Fatalf("WriteFrame error: %v", err)
	}

	readResult := ReadFrame(&buf)
	got, err := readResult.Get()
	if err != nil {
		t.Fatalf("ReadFrame error: %v", err)
	}

	if string(got.Payload) != string(original.Payload) {
		t.Errorf("payload = %q, want %q", got.Payload, original.Payload)
	}
	if got.Opcode != original.Opcode {
		t.Errorf("opcode = %v, want %v", got.Opcode, original.Opcode)
	}
	if got.FIN != original.FIN {
		t.Errorf("FIN = %v, want %v", got.FIN, original.FIN)
	}
}

// TestWriteFrame_LargePayload は 126バイト以上のペイロードで
// 拡張ペイロード長が正しくエンコードされることを検証する。
func TestWriteFrame_LargePayload(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 300)
	f := Frame{FIN: true, Opcode: OpcodeText, Payload: payload}

	var buf bytes.Buffer
	if _, err := WriteFrame(&buf, f).Get(); err != nil {
		t.Fatalf("WriteFrame error: %v", err)
	}

	got, err := ReadFrame(&buf).Get()
	if err != nil {
		t.Fatalf("ReadFrame error: %v", err)
	}

	if len(got.Payload) != 300 {
		t.Errorf("payload length = %d, want 300", len(got.Payload))
	}
}

// TestWriteFrame_CloseFrame は Close フレームが正しく書き込まれることを検証する。
func TestWriteFrame_CloseFrame(t *testing.T) {
	f := Frame{FIN: true, Opcode: OpcodeClose, Payload: []byte{0x03, 0xe8}} // status 1000

	var buf bytes.Buffer
	if _, err := WriteFrame(&buf, f).Get(); err != nil {
		t.Fatalf("WriteFrame error: %v", err)
	}

	got, err := ReadFrame(&buf).Get()
	if err != nil {
		t.Fatalf("ReadFrame error: %v", err)
	}

	if got.Opcode != OpcodeClose {
		t.Errorf("opcode = %v, want OpcodeClose", got.Opcode)
	}
}
