import crypto from "crypto";
import Redis from "ioredis";
import env from "../env.js";
import logger from "./logger.js";

let client;

const getClient = () => {
  if (client) return client;

  client = new Redis(env.redis.url);
  client.on("connect", () => logger.info("redis connected"));
  client.on("error", (err) => logger.error({ err: err.message }, "redis error"));

  return client;
};

const cookieKey = (domain) => {
  const hash = crypto.createHash("sha256").update(domain).digest("hex").slice(0, 16);
  return `${env.queue.cookiePrefix}${hash}`;
};

export const storeCookies = async (domain, cookies) => {
  const rdb = getClient();
  await rdb.set(cookieKey(domain), JSON.stringify(cookies), "EX", env.queue.cookieTTL);
};

export const pushReply = async (jobId, payload) => {
  const rdb = getClient();
  const key = `${env.queue.replyPrefix}${jobId}`;
  await rdb.lpush(key, JSON.stringify(payload));
  await rdb.expire(key, env.queue.replyTTL);
};

export const popJob = async () => {
  const rdb = getClient();
  const result = await rdb.brpop(env.queue.jobQueue, 0);
  return JSON.parse(result[1]);
};

export const shutdown = async () => {
  if (client) {
    await client.quit();
    client = null;
  }
};
