// datachannel.ts は RTCPeerConnection と DataChannel を管理する。
//
// 責務:
//   - signaling.ts からの signal を RTCPeerConnection に反映する
//   - DataChannel でメッセージを送受信する
//   - 受信メッセージを Hono local API 経由で SQLite に保存する
//   - rtcState / signalingState を更新する
//
// WebRTC 接続の流れ:
//   1. startConnection() でシグナリングサーバーに接続し "waiting" 状態になる
//   2. 相手が同じルームに来たらシグナリングサーバーから "ready" が届く
//   3. ready を受け取った側 (offerer) が Offer を作成して送る
//   4. 相手 (answerer) が Answer を返す
//   5. 両者が ICE candidate を交換して P2P 経路を確立する
//   6. DataChannel が open になったらハンドシェイクを送り "connected" になる

import { signal } from "@preact/signals";
import { connect } from "./signaling";
import { rtcState, signalingState, errorMessage } from "../lib/rtc-signals";
import { localClient } from "../lib/client";
import { notify } from "../db/store";
import { getMyPeerId, getMyName } from "../lib/session";

/**
 * STUN サーバーの設定。
 * ICE (Interactive Connectivity Establishment) が P2P 経路を探す際に使う。
 * STUN サーバーは自分のグローバル IP アドレスを教えてくれる役割を持つ。
 */
const ICE_SERVERS: RTCIceServer[] = [
  { urls: "stun:stun.l.google.com:19302" },
];

/**
 * DataChannel 上でやり取りするペイロードの型。
 * - handshake: 接続直後に相手のピアIDと表示名を交換するために使う
 * - message: チャットメッセージ本体
 */
type DCPayload =
  | { type: "handshake"; peerId: string; name: string }
  | {
      type: "message";
      id: string;
      room_id: string;
      peer_id: string;
      body: string;
      sent_at: number;
    };

/**
 * UI 層が DataChannel を操作するためのインターフェース。
 * `connection` signal にセットされ、ChatPage から参照される。
 */
export type Connection = {
  /** メッセージを DataChannel 経由で相手に送信し、自分の SQLite にも保存する */
  sendMessage: (body: string) => Promise<void>;
  /** DataChannel と RTCPeerConnection を閉じる */
  close: () => void;
};

/**
 * UI から sendMessage / close を呼ぶための Signal。
 * DataChannel が open になると Connection オブジェクトがセットされる。
 * 未接続または切断後は null。
 */
export const connection = signal<Connection | null>(null);

/**
 * WebRTC 接続を開始する。
 * 自分をピアとして SQLite に登録し、シグナリングサーバー経由で相手との P2P 接続を確立する。
 * ChatPage のマウント時に一度だけ呼ばれる。
 */
export async function startConnection(roomId: string): Promise<void> {
  const myPeerId = getMyPeerId();
  const myName = getMyName() ?? "Anonymous";

  // 自分自身を peers テーブルに登録する
  await localClient.peers.$post({
    json: { id: myPeerId, name: myName, connected: 1, joined_at: Date.now() },
  });
  await notify(roomId);

  const pc = new RTCPeerConnection({ iceServers: ICE_SERVERS });
  let dc: RTCDataChannel | null = null;
  let remotePeerId: string | null = null;

  /**
   * offerer / answerer 両方で共通の DataChannel イベントを設定する。
   * - offerer: createDataChannel() で作成したチャンネルを渡す
   * - answerer: ondatachannel イベントで受け取ったチャンネルを渡す
   */
  function setupDataChannel(channel: RTCDataChannel): void {
    dc = channel;

    channel.onopen = () => {
      rtcState.value = "connected";
      // 接続確立直後にハンドシェイクを送り、相手に自分のピアIDと名前を知らせる
      channel.send(
        JSON.stringify({ type: "handshake", peerId: myPeerId, name: myName }),
      );
    };

    channel.onmessage = async ({ data }: MessageEvent<string>) => {
      const payload = JSON.parse(data) as DCPayload;

      if (payload.type === "handshake") {
        // 相手のピア情報を SQLite に登録して表示名を解決できるようにする
        remotePeerId = payload.peerId;
        await localClient.peers.$post({
          json: {
            id: payload.peerId,
            name: payload.name,
            connected: 1,
            joined_at: Date.now(),
          },
        });
        await notify(roomId);
        return;
      }

      // チャットメッセージを SQLite に保存して UI に反映する
      await localClient.messages.$post({ json: payload });
      await notify(roomId);
    };

    channel.onclose = () => {
      rtcState.value = "disconnected";
      connection.value = null;
    };
  }

  rtcState.value = "waiting";

  const sigClient = connect(roomId, {
    onReady: async () => {
      // シグナリングサーバーから "ready" を受け取った側が offerer になり Offer を作成する
      rtcState.value = "connecting";
      const channel = pc.createDataChannel("chat");
      setupDataChannel(channel);

      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      signalingState.value = "have-local-offer";
      sigClient.sendOffer(offer);
    },

    onOffer: async (offer) => {
      // Offer を受け取った側が answerer になり Answer を返す
      rtcState.value = "connecting";
      signalingState.value = "have-remote-offer";
      await pc.setRemoteDescription(offer);
      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);
      signalingState.value = "stable";
      sigClient.sendAnswer(answer);
    },

    onAnswer: async (answer) => {
      // offerer が Answer を受け取り、シグナリングが完了する
      await pc.setRemoteDescription(answer);
      signalingState.value = "stable";
    },

    onCandidate: async (candidate) => {
      try {
        await pc.addIceCandidate(candidate);
      } catch {
        // 稀に setRemoteDescription 前に candidate が届くことがある。無視して問題ない。
      }
    },

    onBye: async () => {
      // 相手が切断したらピアを切断済みにして UI を更新する
      if (remotePeerId) {
        await localClient.peers[":id"].disconnect.$put({
          param: { id: remotePeerId },
        });
        await notify(roomId);
      }
      rtcState.value = "disconnected";
      connection.value = null;
      pc.close();
    },

    onError: () => {
      errorMessage.value = "Signaling error";
      rtcState.value = "disconnected";
    },
  });

  // ICE candidate が見つかるたびにシグナリング経由で相手に送る
  pc.onicecandidate = ({ candidate }) => {
    if (candidate) sigClient.sendCandidate(candidate.toJSON());
  };

  // answerer 側は ondatachannel で offerer が作成した DataChannel を受け取る
  pc.ondatachannel = ({ channel }) => {
    setupDataChannel(channel);
  };

  connection.value = {
    sendMessage: async (body: string) => {
      if (!dc || dc.readyState !== "open") {
        errorMessage.value = "Not connected";
        return;
      }
      const msg: DCPayload & { type: "message" } = {
        type: "message",
        id: crypto.randomUUID(),
        room_id: roomId,
        peer_id: myPeerId,
        body,
        sent_at: Date.now(),
      };
      // 自分の SQLite に先に保存してから相手に送る (送信失敗時も自分の履歴には残る)
      await localClient.messages.$post({ json: msg });
      await notify(roomId);
      dc.send(JSON.stringify(msg));
    },

    close: () => {
      sigClient.close();
      pc.close();
      connection.value = null;
    },
  };
}
