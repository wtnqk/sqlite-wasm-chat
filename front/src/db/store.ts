import { signal } from "@preact/signals";
import { localClient } from "../lib/client";
import type { Message } from "../local-api/routes/messages";
import type { Peer } from "../local-api/routes/peers";

export const messages = signal<Message[]>([]);
export const peers = signal<Peer[]>([]);

// notify は SQLite への書き込み後に呼ぶ。
// Hono local API 経由で再クエリし、Signals を更新する。
// signals を読んでいる全コンポーネントが自動再レンダーされる。
export async function notify(roomId: string): Promise<void> {
  const [msgRes, peerRes] = await Promise.all([
    localClient.messages.$get({ query: { room_id: roomId } }),
    localClient.peers.$get({ query: { connected: "1" } }),
  ]);
  messages.value = await msgRes.json();
  peers.value = await peerRes.json();
}
