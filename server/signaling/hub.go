package signaling

import (
	"fmt"
	"sync"

	"github.com/samber/mo"
)

// Hub は全シグナリングルームを管理するシングルトン。
//
// rooms マップへのアクセスは RWMutex で保護する。
// Room 内のピア操作は Room 自身の Mutex が担うため、
// Hub の Mutex は rooms マップの読み書きのみをガードする。
type Hub struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

// JoinResult は Join の結果を保持する。
type JoinResult struct {
	Room  *Room
	Index int
	Full  bool
}

// Join は指定ルームにピアを追加する。
//
// ルームが存在しなければ新規作成する。
// mo.Option を使うことで rooms マップの参照を nil チェックなしに書ける。
func (h *Hub) Join(roomID string, peer *Peer) mo.Result[JoinResult] {
	room := h.getOrCreateRoom(roomID)

	return flatMap(room.AddPeer(peer), func(ar AddResult) mo.Result[JoinResult] {
		return mo.Ok(JoinResult{
			Room:  room,
			Index: ar.Index,
			Full:  ar.Full,
		})
	})
}

// Leave はピアをルームから取り除き、ルームが空になれば削除する。
func (h *Hub) Leave(roomID string, index int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[roomID]
	if !ok {
		return
	}

	room.Remove(index)

	if room.isEmpty() {
		delete(h.rooms, roomID)
	}
}

// RoomCount は現在存在するルーム数を返す。テスト用。
func (h *Hub) RoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// getOrCreateRoom はルームを取得または新規作成する。
// mo.Option で既存ルームの有無を表現し、nil チェックを排除する。
func (h *Hub) getOrCreateRoom(roomID string) *Room {
	// まず読み取りロックで存在確認
	// rooms マップへのアクセスは常にゼロ値 (nil) を返すため TupleToOption で Option に変換する
	h.mu.RLock()
	existing := mo.TupleToOption(h.rooms[roomID], h.rooms[roomID] != nil)
	h.mu.RUnlock()

	if existing.IsPresent() {
		return existing.MustGet()
	}

	// 存在しなければ書き込みロックで作成
	// ロック取得の間に他の goroutine が作成する可能性があるため再確認する
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[roomID]; ok {
		return room
	}

	room := &Room{id: roomID}
	h.rooms[roomID] = room
	return room
}

// validateRoomID は roomID が空でないことを検証する。
func validateRoomID(roomID string) mo.Result[string] {
	if roomID == "" {
		return mo.Err[string](fmt.Errorf("signaling: room ID must not be empty"))
	}
	return mo.Ok(roomID)
}
