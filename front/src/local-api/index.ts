import { Hono } from "hono";
import { messages } from "./routes/messages";
import { peers } from "./routes/peers";

const app = new Hono();

app.route("/messages", messages);
app.route("/peers", peers);

export { app };
export type AppType = typeof app;
