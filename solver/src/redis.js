import Redis from "ioredis";
import env from "../env.js";
import logger from "./logger.js";

let client;
let queueClient;

const createClient = (name) => {
  const c = new Redis(env.redis.url);
  c.on("connect", () => logger.info({ name }, "redis connected"));
  c.on("error", (err) => logger.error({ name, err: err.message }, "redis error"));
  return c;
};

const getClient = () => {
  if (!client) client = createClient("cmd");
  return client;
};

const getQueueClient = () => {
  if (!queueClient) queueClient = createClient("queue");
  return queueClient;
};

const buildKey = (prefix, value) => `${prefix}:${value}`;

export const storeCookies = async (domain, result) => {
  const rdb = getClient();
  const key = buildKey("cookies", domain);
  const data = JSON.stringify(result);
  await rdb.lpush(key, data);
  await rdb.expire(key, env.queue.cookieTTL);
  logger.info({ domain, count: result.cookies?.length ?? 0 }, "stored cookies");
};

export const popJob = async () => {
  const rdb = getQueueClient();
  const result = await rdb.brpop(env.queue.jobQueue, 0);
  return JSON.parse(result[1]);
};

export const isActiveJob = async (domain, jobId) => {
  const rdb = getClient();
  const current = await rdb.get(buildKey("solving", domain));
  return current === jobId;
};

export const shutdown = async () => {
  if (queueClient) {
    queueClient.disconnect();
    queueClient = null;
  }
  if (client) {
    client.disconnect();
    client = null;
  }
};
