// @preact/signals の signal はグローバルな状態変数を作る仕組み。
// React の useState と異なり、コンポーネントの外でも定義できる。
import { signal } from "@preact/signals";
import { init } from "./db";
import { getRoomId } from "./lib/session";
import { JoinPage } from "./components/JoinPage";
import { ChatPage } from "./components/ChatPage";

// signal(初期値) で「リアクティブな変数」を作成する。
// .value を読み書きすると、それを参照しているコンポーネントが自動で再描画される。
const ready = signal(false);          // DBの初期化が完了したか
const initError = signal<string | null>(null); // 初期化中に発生したエラーメッセージ

// モジュール読み込み時に一度だけ SQLite を初期化する。
// signal で管理することで useEffect なしに非同期ロードを表現できる。
init()
  .then(() => { ready.value = true; })           // 成功 → ready を true にして再描画を促す
  .catch((e: unknown) => { initError.value = String(e); }); // 失敗 → エラー文字列を保存

// App はアプリ全体のルートコンポーネント。
// Preact はこの関数の戻り値（JSX）を DOM に反映する。
export function App() {
  // signal の .value を読むと、値が変わったとき自動で再描画がかかる。
  if (initError.value) {
    // エラーがある場合は全画面でエラーメッセージを表示して終了。
    return (
      <div class="min-h-screen bg-gray-950 flex items-center justify-center">
        <p class="text-red-400 text-sm">Failed to initialize: {initError.value}</p>
      </div>
    );
  }

  if (!ready.value) {
    // DB がまだ初期化中の場合はローディング画面を表示。
    return (
      <div class="min-h-screen bg-gray-950 flex items-center justify-center">
        <p class="text-gray-500 text-sm">Loading...</p>
      </div>
    );
  }

  // セッションにルームIDが保存されていればチャット画面、なければ参加画面を表示。
  const roomId = getRoomId();
  return roomId ? <ChatPage roomId={roomId} /> : <JoinPage />;
}
