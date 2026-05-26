import { Hono } from "hono";
import { getDb } from "../../db";

export type Message = {
  id: string;
  room_id: string;
  peer_id: string;
  body: string;
  sent_at: number;
  read_at: number | null;
};

const messages = new Hono();

// GET /messages?room_id=...
messages.get("/", (c) => {
  const roomId = c.req.query("room_id");
  if (!roomId) return c.json({ error: "room_id is required" }, 400);

  const db = getDb();
  const rows = db.selectObjects<Message>(
    "SELECT * FROM messages WHERE room_id = ? ORDER BY sent_at ASC",
    [roomId],
  );
  return c.json(rows);
});

// POST /messages
messages.post("/", async (c) => {
  const { id, room_id, peer_id, body, sent_at } = await c.req.json<Message>();
  const db = getDb();
  db.exec(
    "INSERT INTO messages (id, room_id, peer_id, body, sent_at) VALUES (?,?,?,?,?)",
    { bind: [id, room_id, peer_id, body, sent_at] },
  );
  return c.json({ ok: true }, 201);
});

// PUT /messages/:id/read  既読にする
messages.put("/:id/read", (c) => {
  const db = getDb();
  db.exec("UPDATE messages SET read_at = ? WHERE id = ?", {
    bind: [Date.now(), c.req.param("id")],
  });
  return c.json({ ok: true });
});

export { messages };
