package ws

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/samber/mo"
)

// Opcode は RFC 6455 Section 5.2 で定義されたフレームの種別。
// 0x0〜0x7 はデータフレーム、0x8〜0xF はコントロールフレーム。
type Opcode byte

const (
	// OpcodeText は RFC 6455 Section 5.2 で定義されたテキストフレーム (0x1)。
	// ペイロードは UTF-8 でなければならない (Section 5.6)。
	OpcodeText Opcode = 0x1

	// OpcodeBinary はバイナリフレーム (0x2)。
	OpcodeBinary Opcode = 0x2

	// OpcodeClose は RFC 6455 Section 5.5.1 で定義されたクローズフレーム (0x8)。
	// 受信したら同じクローズフレームをエコーバックしてからコネクションを閉じる。
	OpcodeClose Opcode = 0x8

	// OpcodePing は RFC 6455 Section 5.5.2 で定義された生存確認フレーム (0x9)。
	// 受信したら Pong フレームで応答しなければならない。
	OpcodePing Opcode = 0x9

	// OpcodePong は Ping への応答フレーム (0xA)。
	OpcodePong Opcode = 0xA
)

// Frame は RFC 6455 Section 5.2 で定義されたWebSocketフレームを表す。
//
// フレームのワイヤーフォーマット:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-------+-+-------------+-------------------------------+
//	|F|R|R|R| opcode|M| Payload len |    Extended payload length    |
//	|I|S|S|S|  (4)  |A|     (7)    |             (16/64)           |
//	|N|V|V|V|       |S|             |   (if payload len==126/127)   |
//	+-+-+-+-+-------+-+-------------+-------------------------------+
//	|Masking-key (if MASK=1)        |          Payload Data         |
//	+-------------------------------+ - - - - - - - - - - - - - - -+
//	:                     Payload Data continued ...                :
//	+---------------------------------------------------------------+
type Frame struct {
	// FIN は RFC 6455 Section 5.2 の FIN ビット。
	// true のときこのフレームがメッセージの最終フレームであることを示す。
	// フラグメンテーションを使わない場合は常に true。
	FIN bool

	Opcode  Opcode
	Payload []byte
}

// ReadFrame は io.Reader からWebSocketフレームを1つ読み込む。
//
// RFC 6455 Section 5.2 のフレームフォーマットに従って解析する。
// mo.Result[Frame] を返すことでエラーを値として扱い、呼び出し元でのチェーンを可能にする。
//
// samber/mo v1 の FlatMap は同型間 (Result[T] → Result[T]) のみサポートする。
// Go がまだ higher-kinded types を持たないためで、
// 異なる型への変換には flatMap ヘルパーを使う。
func ReadFrame(r io.Reader) mo.Result[Frame] {
	return flatMap(readHeader(r), func(h frameHeader) mo.Result[Frame] {
		return readPayload(r, h)
	})
}

// flatMap は mo.Result[A] を mo.Result[B] に変換するヘルパー。
//
// samber/mo v1 の Result.FlatMap は Result[T] → Result[T] のみのため、
// 型をまたぐチェーン (frameHeader → Frame など) はこれを使う。
func flatMap[A, B any](r mo.Result[A], fn func(A) mo.Result[B]) mo.Result[B] {
	v, err := r.Get()
	if err != nil {
		return mo.Err[B](err)
	}
	return fn(v)
}

// frameHeader はフレームヘッダーの解析結果を一時的に保持する内部型。
type frameHeader struct {
	fin        bool
	opcode     Opcode
	masked     bool
	payloadLen uint64
	maskingKey [4]byte
}

// readHeader は RFC 6455 Section 5.2 に従ってフレームヘッダーを読み込む。
//
// ヘッダーは最小2バイトで構成され、ペイロード長によって
// 追加で 2 または 8 バイトが続く場合がある。
func readHeader(r io.Reader) mo.Result[frameHeader] {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return mo.Err[frameHeader](fmt.Errorf("ws: failed to read frame header: %w", err))
	}

	h := frameHeader{}

	// 1バイト目: FIN(1bit) + RSV1-3(3bit) + Opcode(4bit)
	// RFC 6455 Section 5.2: RSV1-3 は拡張なしの場合 0 でなければならない
	h.fin = (buf[0] & 0x80) != 0
	h.opcode = Opcode(buf[0] & 0x0F)

	// 2バイト目: MASK(1bit) + Payload length(7bit)
	h.masked = (buf[1] & 0x80) != 0
	initialLen := uint64(buf[1] & 0x7F)

	// RFC 6455 Section 5.2: ペイロード長のエンコーディング
	//   0〜125:  その値がペイロード長
	//   126:     続く2バイト (uint16) がペイロード長
	//   127:     続く8バイト (uint64) がペイロード長
	switch initialLen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return mo.Err[frameHeader](fmt.Errorf("ws: failed to read extended payload length (16bit): %w", err))
		}
		h.payloadLen = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return mo.Err[frameHeader](fmt.Errorf("ws: failed to read extended payload length (64bit): %w", err))
		}
		h.payloadLen = binary.BigEndian.Uint64(ext)
	default:
		h.payloadLen = initialLen
	}

	// RFC 6455 Section 5.3:
	// クライアントからサーバーへのフレームは必ずマスクされなければならない。
	// マスクキーは4バイトで、フレームごとにランダムに選ばれる。
	if h.masked {
		if _, err := io.ReadFull(r, h.maskingKey[:]); err != nil {
			return mo.Err[frameHeader](fmt.Errorf("ws: failed to read masking key: %w", err))
		}
	}

	return mo.Ok(h)
}

// readPayload はヘッダー解析済みの frameHeader を受け取り、
// ペイロードを読み込んでアンマスクした Frame を返す。
func readPayload(r io.Reader, h frameHeader) mo.Result[Frame] {
	payload := make([]byte, h.payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return mo.Err[Frame](fmt.Errorf("ws: failed to read payload: %w", err))
	}

	// RFC 6455 Section 5.3: マスク解除
	// マスクされたデータの各バイトを maskingKey[i % 4] と XOR する。
	// 同じ操作でマスクとアンマスクの両方を行える (XORの対称性)。
	if h.masked {
		for i := range payload {
			payload[i] ^= h.maskingKey[i%4]
		}
	}

	return mo.Ok(Frame{
		FIN:     h.fin,
		Opcode:  h.opcode,
		Payload: payload,
	})
}

// WriteFrame は RFC 6455 Section 5.2 に従ってフレームを io.Writer に書き込む。
//
// RFC 6455 Section 5.1:
// サーバーからクライアントへのフレームはマスクしてはならない。
// (クライアント→サーバーとは逆の規則)
func WriteFrame(w io.Writer, f Frame) mo.Result[struct{}] {
	header := make([]byte, 0, 10)

	// 1バイト目: FIN + Opcode
	b0 := byte(f.Opcode)
	if f.FIN {
		b0 |= 0x80
	}
	header = append(header, b0)

	// 2バイト目以降: MASK=0 (サーバーはマスクしない) + ペイロード長
	l := len(f.Payload)
	switch {
	case l <= 125:
		header = append(header, byte(l))
	case l <= 65535:
		// RFC 6455 Section 5.2: 126〜65535バイトは 126 + 2バイト長
		header = append(header, 126)
		header = binary.BigEndian.AppendUint16(header, uint16(l))
	default:
		// RFC 6455 Section 5.2: 65536バイト以上は 127 + 8バイト長
		header = append(header, 127)
		header = binary.BigEndian.AppendUint64(header, uint64(l))
	}

	if _, err := w.Write(header); err != nil {
		return mo.Err[struct{}](fmt.Errorf("ws: failed to write frame header: %w", err))
	}
	if _, err := w.Write(f.Payload); err != nil {
		return mo.Err[struct{}](fmt.Errorf("ws: failed to write frame payload: %w", err))
	}

	return mo.Ok(struct{}{})
}
