import { signal } from "@preact/signals";
import { init } from "./db";
import { getRoomId } from "./lib/session";
import { JoinPage } from "./components/JoinPage";
import { ChatPage } from "./components/ChatPage";

const ready = signal(false);
const initError = signal<string | null>(null);

// アプリ起動時に SQLite を初期化する。
// signal で管理することで useEffect なしに非同期ロードを表現できる。
init()
  .then(() => { ready.value = true; })
  .catch((e: unknown) => { initError.value = String(e); });

export function App() {
  if (initError.value) {
    return (
      <div class="min-h-screen bg-gray-950 flex items-center justify-center">
        <p class="text-red-400 text-sm">Failed to initialize: {initError.value}</p>
      </div>
    );
  }

  if (!ready.value) {
    return (
      <div class="min-h-screen bg-gray-950 flex items-center justify-center">
        <p class="text-gray-500 text-sm">Loading...</p>
      </div>
    );
  }

  const roomId = getRoomId();
  return roomId ? <ChatPage roomId={roomId} /> : <JoinPage />;
}
