package signaling

import (
	"fmt"

	"github.com/samber/mo"
	"sqlite-wasm-chat/server/ws"
)

// sendBufSize は Peer の送信チャネルのバッファサイズ。
// シグナリングのメッセージ数は少ないため小さくてよい。
const sendBufSize = 16

// Peer は WebSocket で接続している1ピアを表す。
//
// 受信は Handler の読み取りループが担い、
// 送信は send チャネルを通じて writeLoop が非同期で行う。
// この分離により、遅いピアへの書き込みが他の処理をブロックしない。
type Peer struct {
	conn *ws.Conn
	send chan []byte
	done chan struct{} // Close() で閉じ、writeLoop の終了を通知する
}

func newPeer(conn *ws.Conn) *Peer {
	return &Peer{
		conn: conn,
		send: make(chan []byte, sendBufSize),
		done: make(chan struct{}),
	}
}

// Send は data を送信キューに積む。
//
// done チャネルが閉じていれば (= ピアが切断済み) mo.Err を返す。
// バッファが満杯の場合も mo.Err とし、古いメッセージを上書きしない。
// mo.Result を返すことで呼び出し元が FlatMap チェーンを組める。
func (p *Peer) Send(data []byte) mo.Result[struct{}] {
	select {
	case p.send <- data:
		return mo.Ok(struct{}{})
	case <-p.done:
		return mo.Err[struct{}](fmt.Errorf("signaling: peer already disconnected"))
	default:
		return mo.Err[struct{}](fmt.Errorf("signaling: peer send buffer full"))
	}
}

// writeLoop は send チャネルを監視し、メッセージを WebSocket に書き込む。
// Handler の goroutine とは別に起動する。
// done チャネルが閉じられると終了する。
func (p *Peer) writeLoop() {
	for {
		select {
		case data := <-p.send:
			if _, err := p.conn.WriteMessage(data).Get(); err != nil {
				return
			}
		case <-p.done:
			return
		}
	}
}

// close は done チャネルを閉じて writeLoop を終了させ、WebSocket を閉じる。
// 複数回呼ばれても panic しないよう done の close は一度だけ行う。
func (p *Peer) close() {
	select {
	case <-p.done:
		// already closed
	default:
		close(p.done)
	}
	p.conn.Close()
}
