import { signal, computed } from "@preact/signals";

// RtcState は WebRTC 接続のライフサイクルを表す。
//
//   idle ──► waiting ──► connecting ──► connected
//                                           │
//                                      disconnected
//
// idle:         起動直後。接続操作前
// waiting:      シグナリングサーバーに接続し、相手を待っている (offerer 側)
// connecting:   offer/answer 交換済み、ICE negotiation 中
// connected:    DataChannel が open になった
// disconnected: 相手が切断、またはエラー
export type RtcState =
  | "idle"
  | "waiting"
  | "connecting"
  | "connected"
  | "disconnected";

// SignalingState は offer/answer 交換フェーズを表す。
// RTCPeerConnection.signalingState と対応している。
export type SignalingState =
  | "idle"
  | "have-local-offer"
  | "have-remote-offer"
  | "stable";

export const rtcState = signal<RtcState>("idle");
export const signalingState = signal<SignalingState>("idle");
export const errorMessage = signal<string | null>(null);

// isConnected は DataChannel が使える状態かを返す computed signal。
// コンポーネントで送信ボタンの活性/非活性などに使う。
export const isConnected = computed(() => rtcState.value === "connected");
