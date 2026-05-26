// signaling.ts は Go シグナリングサーバーとの WebSocket 接続を管理する。
//
// 責務: signal の送受信のみ。
// RTCPeerConnection の操作や状態更新は呼び出し元 (datachannel.ts) が行う。

// Signal はサーバーとのワイヤーフォーマット。Go 側の Signal 型と対応している。
export type Signal =
  | { type: "ready" }
  | { type: "bye" }
  | { type: "offer"; payload: RTCSessionDescriptionInit }
  | { type: "answer"; payload: RTCSessionDescriptionInit }
  | { type: "candidate"; payload: RTCIceCandidateInit };

export type SignalingHandlers = {
  onReady: () => void;
  onOffer: (offer: RTCSessionDescriptionInit) => void;
  onAnswer: (answer: RTCSessionDescriptionInit) => void;
  onCandidate: (candidate: RTCIceCandidateInit) => void;
  onBye: () => void;
  onError: (err: Event) => void;
};

export type SignalingClient = {
  sendOffer: (offer: RTCSessionDescriptionInit) => void;
  sendAnswer: (answer: RTCSessionDescriptionInit) => void;
  sendCandidate: (candidate: RTCIceCandidateInit) => void;
  close: () => void;
};

// VITE_SIGNALING_URL が未設定なら開発用のデフォルトを使う
const SIGNALING_URL =
  import.meta.env["VITE_SIGNALING_URL"] ?? "ws://localhost:8080";

// connect は roomId の WebSocket エンドポイントに接続し、
// SignalingClient を返す。
// handlers に各シグナル種別の処理を渡す。
export function connect(
  roomId: string,
  handlers: SignalingHandlers,
): SignalingClient {
  const ws = new WebSocket(`${SIGNALING_URL}/ws/${roomId}`);

  ws.onmessage = (e: MessageEvent<string>) => {
    const sig = JSON.parse(e.data) as Signal;
    switch (sig.type) {
      case "ready":
        handlers.onReady();
        break;
      case "offer":
        handlers.onOffer(sig.payload);
        break;
      case "answer":
        handlers.onAnswer(sig.payload);
        break;
      case "candidate":
        handlers.onCandidate(sig.payload);
        break;
      case "bye":
        handlers.onBye();
        break;
    }
  };

  ws.onerror = handlers.onError;

  const send = (sig: Signal) => ws.send(JSON.stringify(sig));

  return {
    sendOffer: (offer) => send({ type: "offer", payload: offer }),
    sendAnswer: (answer) => send({ type: "answer", payload: answer }),
    sendCandidate: (candidate) => send({ type: "candidate", payload: candidate }),
    close: () => ws.close(),
  };
}
