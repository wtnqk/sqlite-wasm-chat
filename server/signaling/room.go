package signaling

import (
	"fmt"
	"sync"

	"github.com/samber/mo"
)

// Room は1つのシグナリングルームを表す。
// peers[0] が offerer (先に入室したピア)、peers[1] が answerer。
// 最大2ピアまで収容できる。
type Room struct {
	id    string
	peers [2]*Peer
	mu    sync.Mutex
}

// AddResult は AddPeer の結果を保持する。
type AddResult struct {
	// Index はこのピアに割り当てられたインデックス (0 or 1)。
	Index int
	// Full は true のとき、このピアの追加でルームが満員になったことを示す。
	// Handler はこれを見て peers[0] に SignalReady を送信する。
	Full bool
}

// AddPeer はルームにピアを追加する。
//
// ルームが満員 (peers[0] と peers[1] が両方存在) の場合はエラーを返す。
// mo.Result を返すことで呼び出し元が FlatMap チェーンを組める。
func (r *Room) AddPeer(p *Peer) mo.Result[AddResult] {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch {
	case r.peers[0] == nil:
		r.peers[0] = p
		return mo.Ok(AddResult{Index: 0, Full: false})
	case r.peers[1] == nil:
		r.peers[1] = p
		return mo.Ok(AddResult{Index: 1, Full: true})
	default:
		return mo.Err[AddResult](fmt.Errorf("signaling: room %q is full", r.id))
	}
}

// Other は指定インデックスの「相手側」ピアを返す。
//
// mo.Option を返すことで、相手がまだ入室していないケースを
// nil チェックなしに表現できる。
func (r *Room) Other(myIndex int) mo.Option[*Peer] {
	r.mu.Lock()
	defer r.mu.Unlock()

	other := r.peers[1-myIndex]
	if other == nil {
		return mo.None[*Peer]()
	}
	return mo.Some(other)
}

// Remove は指定インデックスのピアをルームから取り除く。
func (r *Room) Remove(index int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[index] = nil
}

// isEmpty は両方のピアが nil のとき true を返す。
// Hub がルームを削除すべきかどうかの判断に使う。
func (r *Room) isEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.peers[0] == nil && r.peers[1] == nil
}

// firstPeer は peers[0] を Option で返す。
// 2人目が入室したとき、1人目への SignalReady 送信に使う。
func (r *Room) firstPeer() mo.Option[*Peer] {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.peers[0] == nil {
		return mo.None[*Peer]()
	}
	return mo.Some(r.peers[0])
}
