package ws

import (
	"fmt"
	"net"

	"github.com/samber/mo"
)

// Conn はWebSocket接続を表す高レベルの型。
//
// RFC 6455 Section 1.3:
// ハンドシェイク完了後、クライアントとサーバーはフレームを双方向に送受信できる。
// Conn はその生の net.Conn をラップし、フレームの読み書きを抽象化する。
type Conn struct {
	conn net.Conn
}

// NewConn は Upgrade で取得した net.Conn から Conn を生成する。
func NewConn(c net.Conn) *Conn {
	return &Conn{conn: c}
}

// ReadMessage は次のデータフレーム (Text or Binary) を受信するまで読み進める。
//
// RFC 6455 Section 5.5 のコントロールフレームを透過的に処理する:
//   - Ping → 即座に Pong を返す (Section 5.5.3)
//   - Close → クローズハンドシェイクを行い net.Conn を閉じる (Section 5.5.1)
//   - Pong → 無視 (unsolicited pong は許可されている, Section 5.5.3)
//
// データフレームを受信した場合は (opcode, payload) を返す。
func (c *Conn) ReadMessage() mo.Result[Message] {
	for {
		result := ReadFrame(c.conn)
		frame, err := result.Get()
		if err != nil {
			return mo.Err[Message](err)
		}

		switch frame.Opcode {
		case OpcodeText, OpcodeBinary:
			return mo.Ok(Message{Opcode: frame.Opcode, Data: frame.Payload})

		case OpcodePing:
			// RFC 6455 Section 5.5.3:
			// Ping を受け取ったら、同じペイロードで Pong を返さなければならない。
			if _, err := WriteFrame(c.conn, Frame{FIN: true, Opcode: OpcodePong, Payload: frame.Payload}).Get(); err != nil {
				return mo.Err[Message](fmt.Errorf("ws: failed to send pong: %w", err))
			}

		case OpcodeClose:
			// RFC 6455 Section 5.5.1:
			// Close フレームを受け取ったらエコーバックしてからコネクションを閉じる。
			// ステータスコードが含まれる場合は同じコードを返す。
			_, _ = WriteFrame(c.conn, Frame{FIN: true, Opcode: OpcodeClose, Payload: frame.Payload}).Get()
			c.conn.Close()
			return mo.Err[Message](fmt.Errorf("ws: connection closed by peer"))

		case OpcodePong:
			// RFC 6455 Section 5.5.3:
			// unsolicited Pong (こちらがPingを送っていないのに来たPong) は無視してよい。
		}
	}
}

// WriteMessage はテキストメッセージを送信する。
//
// RFC 6455 Section 6.1: メッセージ送信手順
// フラグメンテーションを使わない場合、1フレーム = 1メッセージ。
// FIN ビットを true にしてメッセージの終端を示す。
func (c *Conn) WriteMessage(data []byte) mo.Result[struct{}] {
	return WriteFrame(c.conn, Frame{
		FIN:     true,
		Opcode:  OpcodeText,
		Payload: data,
	})
}

// Close は RFC 6455 Section 5.5.1 に従ったクリーンなクローズを行う。
//
// クローズシーケンス:
//  1. Close フレームを送信
//  2. 相手からの Close フレームを待つ (ここでは簡略化して省略)
//  3. TCP コネクションを閉じる
func (c *Conn) Close() error {
	_, _ = WriteFrame(c.conn, Frame{FIN: true, Opcode: OpcodeClose}).Get()
	return c.conn.Close()
}

// Message は ReadMessage の戻り値。Opcode とペイロードを保持する。
type Message struct {
	Opcode Opcode
	Data   []byte
}
