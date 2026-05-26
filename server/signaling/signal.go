// Package signaling は WebRTC シグナリングの中継ロジックを実装する。
//
// シグナリングサーバーの唯一の責務は offer/answer/candidate を
// 正しい相手に届けることであり、P2P接続確立後は関与しない。
package signaling

import (
	"encoding/json"
	"fmt"

	"github.com/samber/mo"
)

// SignalType は WebRTC シグナリングで使用するメッセージ種別。
type SignalType string

const (
	// SignalOffer は RFC 8829 (JSEP) Section 4.1.1 で定義される offer SDP。
	// 接続を開始する側 (offerer) が生成して相手に送る。
	SignalOffer SignalType = "offer"

	// SignalAnswer は RFC 8829 Section 4.1.2 で定義される answer SDP。
	// offer を受け取った側 (answerer) が生成して返す。
	SignalAnswer SignalType = "answer"

	// SignalCandidate は RFC 8445 (ICE) で定義される ICE candidate。
	// Trickle ICE (RFC 8838) では offer/answer と並行して逐次送られる。
	SignalCandidate SignalType = "candidate"

	// SignalReady はサーバーが生成するメタシグナル。
	// 2人目のピアが入室したとき、1人目のピアに通知する。
	// offer 生成のトリガーとして使う。
	SignalReady SignalType = "ready"

	// SignalBye はサーバーが生成するメタシグナル。
	// 相手ピアが切断したとき、残ったピアに通知する。
	SignalBye SignalType = "bye"
)

// allowedFromClient は クライアントから受け付けるシグナル種別。
// ready / bye はサーバーが生成するため、クライアントから受け取ってはならない。
var allowedFromClient = map[SignalType]bool{
	SignalOffer:     true,
	SignalAnswer:    true,
	SignalCandidate: true,
}

// Signal はシグナリングメッセージのワイヤーフォーマット。
//
// Payload は種別ごとに異なる構造を持つため json.RawMessage で保持し、
// サーバーは内容を解釈せずそのまま相手に転送する。
type Signal struct {
	Type    SignalType      `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// readySignal と byeSignal はサーバーが送出する定型メッセージ。
// 毎回エンコードするコストを避けるため起動時に生成する。
var (
	readySignal = mustMarshal(Signal{Type: SignalReady})
	byeSignal   = mustMarshal(Signal{Type: SignalBye})
)

// parseSignal は JSON バイト列を Signal に変換する。
func parseSignal(data []byte) mo.Result[Signal] {
	var sig Signal
	return mo.TupleToResult(sig, json.Unmarshal(data, &sig))
}

// validateClientSignal はクライアントから送られたシグナルが
// allowedFromClient に含まれることを検証する。
func validateClientSignal(sig Signal) mo.Result[Signal] {
	if !allowedFromClient[sig.Type] {
		return mo.Err[Signal](fmt.Errorf("signaling: type %q is not allowed from client", sig.Type))
	}
	return mo.Ok(sig)
}

// relay はクライアントから受け取った生バイト列を検証し、
// Option で渡された相手ピアに転送する。
//
// mo.Option を使うことで「相手がいない」ケースを nil チェックなしに扱える。
func relay(raw []byte, other mo.Option[*Peer]) mo.Result[struct{}] {
	validated := flatMap(parseSignal(raw), validateClientSignal)

	return flatMap(validated, func(sig Signal) mo.Result[struct{}] {
		b, err := json.Marshal(sig)
		if err != nil {
			return mo.Err[struct{}](fmt.Errorf("signaling: marshal error: %w", err))
		}
		// mo.Option には OkOr がないため optionToResult ヘルパーで変換する
		peerResult := optionToResult(other, fmt.Errorf("signaling: no peer in room to relay to"))
		return flatMap(peerResult, func(p *Peer) mo.Result[struct{}] {
			return p.Send(b)
		})
	})
}

// optionToResult は mo.Option[T] を mo.Result[T] に変換する。
// samber/mo v1 の Option に OkOr が存在しないため必要。
func optionToResult[T any](o mo.Option[T], err error) mo.Result[T] {
	v, ok := o.Get()
	if !ok {
		return mo.Err[T](err)
	}
	return mo.Ok(v)
}

// flatMap は mo.Result[A] → mo.Result[B] の型変換チェーン用ヘルパー。
// samber/mo v1 の FlatMap は同型 (Result[T]→Result[T]) のみのため必要。
func flatMap[A, B any](r mo.Result[A], fn func(A) mo.Result[B]) mo.Result[B] {
	v, err := r.Get()
	if err != nil {
		return mo.Err[B](err)
	}
	return fn(v)
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
