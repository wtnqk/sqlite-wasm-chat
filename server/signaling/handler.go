package signaling

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/samber/mo"
	"sqlite-wasm-chat/server/ws"
)

// setup はハンドラーの初期化フェーズをまとめた内部型。
// HTTP→WS アップグレード、ピア生成、Hub 登録の3ステップをチェーンで繋ぐ。
type setup struct {
	conn *ws.Conn
	peer *Peer
	jr   JoinResult
}

// Handler は WebSocket シグナリングエンドポイントの HTTP ハンドラーを返す。
//
// エンドポイント: GET /ws/{room_id}
//
// 処理フロー:
//  1. roomID 検証 → HTTP→WS アップグレード → Hub 登録を mo チェーンで実行
//  2. 2人目が入室したとき、1人目に SignalReady を送信
//  3. 受信ループ: クライアントからのシグナルを相手ピアに中継
//  4. 切断時: 相手に SignalBye を送信し、Hub からピアを削除
func Handler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := r.PathValue("room_id")

		// setup フェーズを mo チェーンで記述する。
		// 各ステップが mo.Result を返すため、途中でエラーになると以降はスキップされる。
		result := flatMap(validateRoomID(roomID), func(_ string) mo.Result[setup] {
			return flatMap(
				mo.TupleToResult(ws.Upgrade(w, r)),
				func(netConn net.Conn) mo.Result[setup] {
					conn := ws.NewConn(netConn)
					peer := newPeer(conn)
					return flatMap(hub.Join(roomID, peer), func(jr JoinResult) mo.Result[setup] {
						return mo.Ok(setup{conn: conn, peer: peer, jr: jr})
					})
				},
			)
		})

		s, err := result.Get()
		if err != nil {
			slog.Warn("setup failed", "room", roomID, "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer s.peer.close()
		defer hub.Leave(roomID, s.jr.Index)

		// 送信ループを別 goroutine で起動
		go s.peer.writeLoop()

		// 2人揃ったとき: 1人目 (offerer) に ready を通知して offer 生成を促す
		if s.jr.Full {
			s.jr.Room.firstPeer().ForEach(func(first *Peer) {
				first.Send(readySignal)
			})
		}

		slog.Info("peer joined", "room", roomID, "index", s.jr.Index)

		// 受信ループ
		// ReadMessage は mo.Result を返すが、ループを継続/終了する制御が必要なため
		// .Get() でアンラップして通常の Go の制御フローに戻す。
		for {
			msg, err := s.conn.ReadMessage().Get()
			if err != nil {
				s.jr.Room.Other(s.jr.Index).ForEach(func(other *Peer) {
					other.Send(byeSignal)
				})
				slog.Info("peer left", "room", roomID, "index", s.jr.Index)
				return
			}

			if _, err := relay(msg.Data, s.jr.Room.Other(s.jr.Index)).Get(); err != nil {
				slog.Warn("relay failed", "room", roomID, "err", err)
			}
		}
	}
}
