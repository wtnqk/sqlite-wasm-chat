package signaling

import (
	"testing"
)

// --- Room テスト ---

// TestRoom_AddPeer_First は1人目の追加が index=0, Full=false になることを検証する。
func TestRoom_AddPeer_First(t *testing.T) {
	room := &Room{id: "r1"}
	peer := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}

	result := room.AddPeer(peer)
	ar, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ar.Index != 0 {
		t.Errorf("index = %d, want 0", ar.Index)
	}
	if ar.Full {
		t.Error("Full should be false for first peer")
	}
}

// TestRoom_AddPeer_Second は2人目の追加が index=1, Full=true になることを検証する。
func TestRoom_AddPeer_Second(t *testing.T) {
	room := &Room{id: "r1"}
	p1 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}
	p2 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}

	room.AddPeer(p1)
	result := room.AddPeer(p2)
	ar, err := result.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ar.Index != 1 {
		t.Errorf("index = %d, want 1", ar.Index)
	}
	if !ar.Full {
		t.Error("Full should be true for second peer")
	}
}

// TestRoom_AddPeer_Full は3人目の追加がエラーになることを検証する。
func TestRoom_AddPeer_Full(t *testing.T) {
	room := &Room{id: "r1"}
	for i := 0; i < 2; i++ {
		room.AddPeer(&Peer{send: make(chan []byte, 1), done: make(chan struct{})})
	}

	result := room.AddPeer(&Peer{send: make(chan []byte, 1), done: make(chan struct{})})
	if result.IsOk() {
		t.Error("expected error for full room, got ok")
	}
}

// TestRoom_Other はインデックスの「相手側」が正しく返ることを検証する。
func TestRoom_Other(t *testing.T) {
	room := &Room{id: "r1"}
	p0 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}
	p1 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}

	room.AddPeer(p0)
	room.AddPeer(p1)

	// index=0 の相手は p1
	other0, ok := room.Other(0).Get()
	if !ok || other0 != p1 {
		t.Error("Other(0) should return p1")
	}

	// index=1 の相手は p0
	other1, ok := room.Other(1).Get()
	if !ok || other1 != p0 {
		t.Error("Other(1) should return p0")
	}
}

// TestRoom_Other_NoPartner は相手がまだ入室していないとき None を返すことを検証する。
func TestRoom_Other_NoPartner(t *testing.T) {
	room := &Room{id: "r1"}
	p0 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}
	room.AddPeer(p0)

	if room.Other(0).IsPresent() {
		t.Error("Other(0) should be None when only one peer is in room")
	}
}

// TestRoom_Remove はピアを削除した後に isEmpty が true になることを検証する。
func TestRoom_Remove(t *testing.T) {
	room := &Room{id: "r1"}
	p0 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}
	room.AddPeer(p0)

	room.Remove(0)
	if !room.isEmpty() {
		t.Error("room should be empty after removing the only peer")
	}
}

// --- Hub テスト ---

// TestHub_Join_CreatesRoom は Join でルームが作成されることを検証する。
func TestHub_Join_CreatesRoom(t *testing.T) {
	hub := NewHub()
	peer := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}

	if _, err := hub.Join("room1", peer).Get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hub.RoomCount() != 1 {
		t.Errorf("room count = %d, want 1", hub.RoomCount())
	}
}

// TestHub_Join_SameRoom は同じルームIDに2人が参加できることを検証する。
func TestHub_Join_SameRoom(t *testing.T) {
	hub := NewHub()
	p1 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}
	p2 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}

	jr1, _ := hub.Join("room1", p1).Get()
	jr2, _ := hub.Join("room1", p2).Get()

	if jr1.Index != 0 || jr1.Full {
		t.Errorf("p1: index=%d full=%v, want index=0 full=false", jr1.Index, jr1.Full)
	}
	if jr2.Index != 1 || !jr2.Full {
		t.Errorf("p2: index=%d full=%v, want index=1 full=true", jr2.Index, jr2.Full)
	}
	// 同じ Room オブジェクトを共有しているはず
	if jr1.Room != jr2.Room {
		t.Error("both peers should share the same room")
	}
}

// TestHub_Join_FullRoom は満員のルームへの参加がエラーになることを検証する。
func TestHub_Join_FullRoom(t *testing.T) {
	hub := NewHub()
	for i := 0; i < 2; i++ {
		hub.Join("room1", &Peer{send: make(chan []byte, 1), done: make(chan struct{})})
	}

	result := hub.Join("room1", &Peer{send: make(chan []byte, 1), done: make(chan struct{})})
	if result.IsOk() {
		t.Error("expected error for full room, got ok")
	}
}

// TestHub_Leave_DeletesEmptyRoom は最後のピアが退室したらルームが削除されることを検証する。
func TestHub_Leave_DeletesEmptyRoom(t *testing.T) {
	hub := NewHub()
	p := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}
	hub.Join("room1", p)

	hub.Leave("room1", 0)

	if hub.RoomCount() != 0 {
		t.Errorf("room count = %d, want 0 after last peer leaves", hub.RoomCount())
	}
}

// TestHub_Leave_RoomRemainsIfPartnerPresent は片方が退室してもルームが残ることを検証する。
func TestHub_Leave_RoomRemainsIfPartnerPresent(t *testing.T) {
	hub := NewHub()
	p1 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}
	p2 := &Peer{send: make(chan []byte, 1), done: make(chan struct{})}
	hub.Join("room1", p1)
	hub.Join("room1", p2)

	hub.Leave("room1", 0) // p1 が退室

	if hub.RoomCount() != 1 {
		t.Errorf("room count = %d, want 1 (p2 still in room)", hub.RoomCount())
	}
}

// TestHub_Leave_UnknownRoom は存在しないルームIDで Leave を呼んでも panic しないことを検証する。
func TestHub_Leave_UnknownRoom(t *testing.T) {
	hub := NewHub()
	hub.Leave("nonexistent", 0) // panic しなければ OK
}

// TestHub_Join_EmptyRoomID は空の roomID がエラーになることを検証する。
func TestHub_Join_EmptyRoomID(t *testing.T) {
	// Hub.Join は validateRoomID を呼ばないが、Handler が呼ぶ。
	// validateRoomID を直接テストする。
	if validateRoomID("").IsOk() {
		t.Error("empty room ID should fail validation")
	}
	if _, err := validateRoomID("valid-room").Get(); err != nil {
		t.Errorf("valid room ID should pass: %v", err)
	}
}
