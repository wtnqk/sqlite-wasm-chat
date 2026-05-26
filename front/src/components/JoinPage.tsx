import { useRef } from "preact/hooks";
import { getMyName, setMyName } from "../lib/session";

export function JoinPage() {
  const nameRef = useRef<HTMLInputElement>(null);
  const roomRef = useRef<HTMLInputElement>(null);

  const handleJoin = () => {
    const name = nameRef.current?.value.trim();
    const room = roomRef.current?.value.trim() || crypto.randomUUID().slice(0, 8);
    if (!name) return;
    setMyName(name);
    window.location.href = `/${room}`;
  };

  return (
    <div class="min-h-screen bg-gray-950 flex items-center justify-center p-4">
      <div class="w-full max-w-sm space-y-6">
        <h1 class="text-2xl font-semibold text-white text-center">P2P Chat</h1>

        <div class="space-y-3">
          <input
            ref={nameRef}
            type="text"
            placeholder="Your name"
            defaultValue={getMyName() ?? ""}
            class="w-full px-4 py-2.5 rounded-lg bg-gray-800 text-white placeholder-gray-500 border border-gray-700 focus:outline-none focus:border-gray-500"
          />
          <input
            ref={roomRef}
            type="text"
            placeholder="Room ID (leave blank to generate)"
            class="w-full px-4 py-2.5 rounded-lg bg-gray-800 text-white placeholder-gray-500 border border-gray-700 focus:outline-none focus:border-gray-500"
          />
          <button
            onClick={handleJoin}
            class="w-full py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-medium transition-colors"
          >
            Join
          </button>
        </div>

        <p class="text-xs text-gray-600 text-center">
          メッセージはサーバーに保存されません
        </p>
      </div>
    </div>
  );
}
