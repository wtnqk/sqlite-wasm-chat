// datachannel.ts は RTCPeerConnection と DataChannel を管理する。
//
// 責務:
//   - signaling.ts からの signal を RTCPeerConnection に反映する
//   - DataChannel でメッセージを送受信する
//   - 受信メッセージを Hono local API 経由で SQLite に保存する
//   - rtcState / signalingState を更新する

import { signal } from "@preact/signals";
import { connect } from "./signaling";
import { rtcState, signalingState, errorMessage } from "../lib/rtc-signals";
import { localClient } from "../lib/client";
import { notify } from "../db/store";
import { getMyPeerId, getMyName } from "../lib/session";

const ICE_SERVERS: RTCIceServer[] = [
  { urls: "stun:stun.l.google.com:19302" },
];

// DataChannel 上でやり取りするペイロードの型
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

export type Connection = {
  sendMessage: (body: string) => Promise<void>;
  close: () => void;
};

// connection は UI から sendMessage / close を呼ぶための Signal。
// DataChannel が open になると値がセットされる。
export const connection = signal<Connection | null>(null);

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

  function setupDataChannel(channel: RTCDataChannel): void {
    dc = channel;

    channel.onopen = () => {
      rtcState.value = "connected";
      // 接続確立直後にハンドシェイクを送り、相手のピアIDと名前を知らせる
      channel.send(
        JSON.stringify({ type: "handshake", peerId: myPeerId, name: myName }),
      );
    };

    channel.onmessage = async ({ data }: MessageEvent<string>) => {
      const payload = JSON.parse(data) as DCPayload;

      if (payload.type === "handshake") {
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

      // chat message → SQLite に保存
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
      // ready を受け取った側が offerer になる
      rtcState.value = "connecting";
      const channel = pc.createDataChannel("chat");
      setupDataChannel(channel);

      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      signalingState.value = "have-local-offer";
      sigClient.sendOffer(offer);
    },

    onOffer: async (offer) => {
      rtcState.value = "connecting";
      signalingState.value = "have-remote-offer";
      await pc.setRemoteDescription(offer);
      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);
      signalingState.value = "stable";
      sigClient.sendAnswer(answer);
    },

    onAnswer: async (answer) => {
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

  // ICE candidate を相手に送る
  pc.onicecandidate = ({ candidate }) => {
    if (candidate) sigClient.sendCandidate(candidate.toJSON());
  };

  // answerer 側は ondatachannel で DataChannel を受け取る
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
      // 自分の SQLite に先に保存してから相手に送る
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
