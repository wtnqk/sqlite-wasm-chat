// useEffect: 副作用 (接続・スクロール) を実行するフック
// useRef: DOM 要素への参照を保持するフック (再描画を起こさない)
import { useEffect, useRef } from "preact/hooks";
// computed: 複数の signal を合成して派生値を作る。依存 signal が変わると自動で再計算される。
import { computed } from "@preact/signals";
import { messages, peers } from "../db/store";
import { rtcState, isConnected, errorMessage } from "../lib/rtc-signals";
import { connection, startConnection } from "../webrtc/datachannel";
import { getMyPeerId } from "../lib/session";

// WebRTC 接続状態を人間向けのラベルに変換するテーブル
const STATUS_LABEL: Record<string, string> = {
  idle: "Idle",
  waiting: "Waiting for peer...",
  connecting: "Connecting...",
  connected: "Connected",
  disconnected: "Disconnected",
};

// 接続状態ごとのテキスト色 (Tailwind クラス)
const STATUS_COLOR: Record<string, string> = {
  idle: "text-gray-500",
  waiting: "text-yellow-400",
  connecting: "text-yellow-400",
  connected: "text-green-400",
  disconnected: "text-red-400",
};

// computed は signal の派生値。(messages, peers) どちらかが変わると自動で再計算される。
// ここではピアIDをピア名に変換し、自分のメッセージかどうかのフラグも付与している。
const messagesWithName = computed(() => {
  const peerMap = new Map(peers.value.map((p) => [p.id, p.name]));
  return messages.value.map((m) => ({
    ...m,
    peerName: peerMap.get(m.peer_id) ?? "Unknown", // IDから名前を引く
    isMine: m.peer_id === getMyPeerId(), //自分が送ったメッセージか
  }));
});

type Props = { roomId: string };

export function ChatPage({ roomId }: Props) {
  // inputRef は <input> DOM 要素を直接操作するために使う (送信後の value クリアなど)
  const inputRef = useRef<HTMLInputElement>(null);
  // bottomRef はメッセージリストの末尾に置いた空 div。scrollIntoView の対象にする。
  const bottomRef = useRef<HTMLDivElement>(null);

  // コンポーネントのマウント時に WebRTC 接続を開始する。
  // 依存配列に roomId を指定しているので roomId が変わっても再接続される。
  // クリーンアップ関数でアンマウント時に接続を閉じる。
  useEffect(() => {
    startConnection(roomId);
    return () => connection.value?.close();
  }, [roomId]);

  // メッセージが増えるたびに最下部へスムーズスクロールする。
  // messages.value.length を依存配列に入れることでメッセージ追加時だけ発火する。
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.value.length]);

  const handleSend = async () => {
    // 入力欄のテキストを取得して前後の空白を除去する
    // ?. は inputRef.current が null のとき undefined を返して例外を防ぐ
    const body = inputRef.current?.value.trim();
    // 空文字列・null・undefined または未接続の場合は何もしない
    if (!body || !isConnected.value) return;
    // ここまで来たら inputRef.current は必ず存在するので ! で断言してクリアする
    inputRef.current!.value = "";
    // WebRTC データチャネル経由で相手にメッセージを送信する
    await connection.value?.sendMessage(body);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    // Shift+Enter は改行のために使うので、Enter 単体のときだけ送信する
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div class="min-h-screen bg-gray-950 flex flex-col">
      {/* Header: 戻るボタン / ルームID / 接続状態 */}
      <header class="flex items-center justify-between px-4 py-3 border-b border-gray-800">
        <div class="flex items-center gap-3">
          <button
            onClick={() => { connection.value?.close(); window.location.href = "/"; }}
            class="text-gray-500 hover:text-white transition-colors text-sm"
          >
            ← Back
          </button>
          <span class="text-gray-400 text-sm font-mono">{roomId}</span>
        </div>
        {/* rtcState が未定義のキーの場合は fallback でグレー表示 */}
        <span class={`text-sm ${STATUS_COLOR[rtcState.value] ?? "text-gray-500"}`}>
          {STATUS_LABEL[rtcState.value] ?? rtcState.value}
        </span>
      </header>

      {/* Error banner: errorMessage が空文字/null のときは何も描画しない */}
      {errorMessage.value && (
        <div class="bg-red-900/50 text-red-300 text-sm px-4 py-2">
          {errorMessage.value}
        </div>
      )}

      {/* Message list: 自分のメッセージは右寄せ、相手は左寄せ */}
      <main class="flex-1 overflow-y-auto px-4 py-4 space-y-3">
        {messagesWithName.value.map((m) => (
          <div
            key={m.id}
            class={`flex flex-col gap-0.5 ${m.isMine ? "items-end" : "items-start"}`}
          >
            <span class="text-xs text-gray-500">{m.peerName}</span>
            <div
              class={`px-3 py-2 rounded-2xl text-sm max-w-xs break-words ${
                m.isMine
                  ? "bg-indigo-600 text-white"      // 自分: 青紫の吹き出し
                  : "bg-gray-800 text-gray-100"     // 相手: グレーの吹き出し
              }`}
            >
              {m.body}
            </div>
          </div>
        ))}
        {/* 末尾スクロール用のアンカー要素 */}
        <div ref={bottomRef} />
      </main>

      {/* Input: 未接続時は入力欄とボタンを disabled にする */}
      <footer class="px-4 py-3 border-t border-gray-800 flex gap-2">
        <input
          ref={inputRef}
          type="text"
          placeholder={isConnected.value ? "Message..." : "Waiting for connection..."}
          disabled={!isConnected.value}
          onKeyDown={handleKeyDown}
          class="flex-1 px-4 py-2.5 rounded-lg bg-gray-800 text-white placeholder-gray-600 border border-gray-700 focus:outline-none focus:border-gray-500 disabled:opacity-40"
        />
        <button
          onClick={handleSend}
          disabled={!isConnected.value}
          class="px-4 py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
        >
          Send
        </button>
      </footer>
    </div>
  );
}
