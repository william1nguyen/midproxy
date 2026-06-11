import Redis from "ioredis";
import env from "../env";
import logger from "./logger";
import type { Job, SolveResult } from "./types";

let client: Redis | null = null;
let queueClient: Redis | null = null;

const createClient = (name: string): Redis => {
  const c = new Redis(env.redis.url);
  c.on("connect", () => logger.info({ name }, "redis connected"));
  c.on("error", (err: Error) =>
    logger.error({ name, err: err.message }, "redis error"),
  );
  return c;
};

const getClient = (): Redis => {
  if (!client) client = createClient("cmd");
  return client;
};

const getQueueClient = (): Redis => {
  if (!queueClient) queueClient = createClient("queue");
  return queueClient;
};

const buildKey = (prefix: string, value: string): string =>
  `${prefix}:${value}`;

export const storeCookies = async (
  domain: string,
  result: SolveResult,
): Promise<void> => {
  const rdb = getClient();
  const key = buildKey("cookies", domain);
  const data = JSON.stringify(result);
  await rdb.lpush(key, data);
  await rdb.expire(key, env.queue.cookieTTL);
  logger.info({ domain, count: result.cookies?.length ?? 0 }, "stored cookies");
};

export const popJob = async (): Promise<Job> => {
  const rdb = getQueueClient();
  const result = await rdb.brpop(env.queue.jobQueue, 0);
  return JSON.parse(result![1]) as Job;
};

export const isActiveJob = async (
  domain: string,
  jobId: string,
): Promise<boolean> => {
  const rdb = getClient();
  const current = await rdb.get(buildKey("solving", domain));
  return current === jobId;
};

export const shutdown = async (): Promise<void> => {
  if (queueClient) {
    queueClient.disconnect();
    queueClient = null;
  }
  if (client) {
    client.disconnect();
    client = null;
  }
};
