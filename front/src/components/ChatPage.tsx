import { useEffect, useRef } from "preact/hooks";
import { computed } from "@preact/signals";
import { messages, peers } from "../db/store";
import { rtcState, isConnected, errorMessage } from "../lib/rtc-signals";
import { connection, startConnection } from "../webrtc/datachannel";
import { getMyPeerId } from "../lib/session";

const STATUS_LABEL: Record<string, string> = {
  idle: "Idle",
  waiting: "Waiting for peer...",
  connecting: "Connecting...",
  connected: "Connected",
  disconnected: "Disconnected",
};

const STATUS_COLOR: Record<string, string> = {
  idle: "text-gray-500",
  waiting: "text-yellow-400",
  connecting: "text-yellow-400",
  connected: "text-green-400",
  disconnected: "text-red-400",
};

// メッセージにピア名を付与した computed
const messagesWithName = computed(() => {
  const peerMap = new Map(peers.value.map((p) => [p.id, p.name]));
  return messages.value.map((m) => ({
    ...m,
    peerName: peerMap.get(m.peer_id) ?? "Unknown",
    isMine: m.peer_id === getMyPeerId(),
  }));
});

type Props = { roomId: string };

export function ChatPage({ roomId }: Props) {
  const inputRef = useRef<HTMLInputElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  // 接続を開始する。useEffect でマウント時に一度だけ実行する。
  useEffect(() => {
    startConnection(roomId);
    return () => connection.value?.close();
  }, [roomId]);

  // 新しいメッセージが来たら一番下にスクロール
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.value.length]);

  const handleSend = async () => {
    const body = inputRef.current?.value.trim();
    if (!body || !isConnected.value) return;
    inputRef.current!.value = "";
    await connection.value?.sendMessage(body);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div class="min-h-screen bg-gray-950 flex flex-col">
      {/* Header */}
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
        <span class={`text-sm ${STATUS_COLOR[rtcState.value] ?? "text-gray-500"}`}>
          {STATUS_LABEL[rtcState.value] ?? rtcState.value}
        </span>
      </header>

      {/* Error banner */}
      {errorMessage.value && (
        <div class="bg-red-900/50 text-red-300 text-sm px-4 py-2">
          {errorMessage.value}
        </div>
      )}

      {/* Message list */}
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
                  ? "bg-indigo-600 text-white"
                  : "bg-gray-800 text-gray-100"
              }`}
            >
              {m.body}
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </main>

      {/* Input */}
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
