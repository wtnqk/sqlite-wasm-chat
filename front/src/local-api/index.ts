import { Hono } from "hono";
import { messages } from "./routes/messages";
import { peers } from "./routes/peers";

// .route() をチェーンすることでルートの型情報が app に積み上がり、
// hc<AppType> がエンドポイントを型安全に認識できる。
const app = new Hono()
  .route("/messages", messages)
  .route("/peers", peers);

export { app };
export type AppType = typeof app;
