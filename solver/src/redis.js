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

const buildKey = (prefix, value) => `${prefix}:${value}`;

export const storeCookies = async (domain, cookies) => {
  const rdb = getClient();
  const key = buildKey("cookies", domain);
  await rdb.lpush(key, JSON.stringify(cookies));
  await rdb.expire(key, env.queue.cookieTTL);
};

export const popJob = async () => {
  const rdb = getClient();
  const result = await rdb.brpop(env.queue.jobQueue, 0);
  return JSON.parse(result[1]);
};

export const shutdown = async () => {
  if (client) {
    client.disconnect();
    client = null;
  }
};
