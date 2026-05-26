// session.ts は SessionStorage へのアクセスを型安全にまとめるユーティリティ。
//
// SessionStorage を選ぶ理由:
//   - ページリロードで復元できる (localStorageと同様)
//   - タブを閉じると消える (別タブと状態が混ざらない)
//   - シグナリングに必要な情報はタブ単位で管理すれば十分

const KEYS = {
  peerId: "chat:peerId",
  peerName: "chat:peerName",
} as const;

// getMyPeerId はセッション内で一意なピアIDを返す。
// 初回アクセス時に crypto.randomUUID() で生成してSessionStorageに保存する。
export function getMyPeerId(): string {
  let id = sessionStorage.getItem(KEYS.peerId);
  if (!id) {
    id = crypto.randomUUID();
    sessionStorage.setItem(KEYS.peerId, id);
  }
  return id;
}

// getMyName は保存済みの表示名を返す。未設定なら null。
export function getMyName(): string | null {
  return sessionStorage.getItem(KEYS.peerName);
}

// setMyName は表示名を保存する。
export function setMyName(name: string): void {
  sessionStorage.setItem(KEYS.peerName, name);
}

// getRoomId は現在のURLパスからルームIDを取得する。
// URL設計: /:roomId
export function getRoomId(): string | null {
  const id = location.pathname.slice(1);
  return id || null;
}
