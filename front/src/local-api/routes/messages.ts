import { Hono } from "hono";
import { getDb } from "../../db";

/**
 * チャットメッセージを表す型。
 * SQLite の messages テーブルの1行に対応する。
 */
export type Message = {
  /** UUID などの一意な識別子 */
  id: string;
  /** どのルームのメッセージか */
  room_id: string;
  /** 送信したピアの ID */
  peer_id: string;
  /** メッセージ本文 */
  body: string;
  /** 送信日時 (Unix ミリ秒) */
  sent_at: number;
  /** 既読日時 (Unix ミリ秒)。未読の場合は null */
  read_at: number | null;
};

/**
 * /messages ルートハンドラー。
 *
 * | メソッド | パス               | 説明                         |
 * | -------- | ------------------ | ---------------------------- |
 * | GET      | /?room_id=...      | ルームのメッセージ一覧を取得 |
 * | POST     | /                  | メッセージを保存             |
 * | PUT      | /:id/read          | メッセージを既読にする       |
 */
const messages = new Hono()
  // GET /messages?room_id=...
  .get("/", (c) => {
    const roomId = c.req.query("room_id");
    if (!roomId) return c.json({ error: "room_id is required" }, 400);

    const db = getDb();
    // @ts-expect-error -- @sqlite.org/sqlite-wasm の型定義が型引数なしで宣言されているが実装は受け取る
    const rows = db.selectObjects<Message>(
      "SELECT * FROM messages WHERE room_id = ? ORDER BY sent_at ASC",
      [roomId],
    ) as Message[];
    return c.json(rows);
  })
  // POST /messages
  .post("/", async (c) => {
    const { id, room_id, peer_id, body, sent_at } = await c.req.json<Message>();
    const db = getDb();
    db.exec(
      "INSERT INTO messages (id, room_id, peer_id, body, sent_at) VALUES (?,?,?,?,?)",
      { bind: [id, room_id, peer_id, body, sent_at] },
    );
    return c.json({ ok: true }, 201);
  })
  // PUT /messages/:id/read  既読にする
  .put("/:id/read", (c) => {
    const db = getDb();
    // read_at に現在時刻を設定して既読状態にする
    db.exec("UPDATE messages SET read_at = ? WHERE id = ?", {
      bind: [Date.now(), c.req.param("id")],
    });
    return c.json({ ok: true });
  });

export { messages };
