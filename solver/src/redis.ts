import Redis from "ioredis";
import env from "../env";
import logger from "./logger";
import type { Job, SolveResult } from "./types";

let client: Redis | null = null;
let queueClient: Redis | null = null;

const STREAM = "stream:solve";
const GROUP = "solvers";
const CONSUMER = `worker-${process.pid}`;

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

export const ensureConsumerGroup = async (): Promise<void> => {
  const rdb = getClient();
  try {
    await rdb.xgroup("CREATE", STREAM, GROUP, "0", "MKSTREAM");
    logger.info({ stream: STREAM, group: GROUP }, "consumer group created");
  } catch (err) {
    const msg = (err as Error).message;
    if (!msg.includes("BUSYGROUP")) throw err;
  }
};

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

export interface StreamJob {
  messageId: string;
  job: Job;
}

export const readJob = async (): Promise<StreamJob | null> => {
  const rdb = getQueueClient();
  const result = await rdb.xreadgroup(
    "GROUP",
    GROUP,
    CONSUMER,
    "BLOCK",
    0,
    "COUNT",
    1,
    "STREAMS",
    STREAM,
    ">",
  );

  if (!result || result.length === 0) return null;

  const [, messages] = result[0] as [string, [string, string[]][]];
  if (messages.length === 0) return null;

  const [messageId, fields] = messages[0];
  const obj: Record<string, string> = {};
  for (let i = 0; i < fields.length; i += 2) {
    obj[fields[i]] = fields[i + 1];
  }

  return { messageId, job: { id: obj.id, url: obj.url } };
};

export const ackJob = async (messageId: string): Promise<void> => {
  const rdb = getClient();
  await rdb.xack(STREAM, GROUP, messageId);
};

export const isActiveJob = async (
  domain: string,
  jobId: string,
): Promise<boolean> => {
  const rdb = getClient();
  const current = await rdb.get(buildKey("solving", domain));
  return current === jobId;
};

export const getRetryCount = async (jobId: string): Promise<number> => {
  const rdb = getClient();
  const val = await rdb.get(buildKey("retry", jobId));
  return val ? parseInt(val, 10) : 0;
};

export const incrementRetry = async (jobId: string): Promise<number> => {
  const rdb = getClient();
  const key = buildKey("retry", jobId);
  const count = await rdb.incr(key);
  await rdb.expire(key, 600);
  return count;
};

export const requeueJob = async (job: Job): Promise<void> => {
  const rdb = getClient();
  await rdb.xadd(STREAM, "*", "id", job.id, "url", job.url);
};

export const pushDeadLetter = async (
  job: Job,
  error: string,
  retries: number,
): Promise<void> => {
  const rdb = getClient();
  const payload = JSON.stringify({
    ...job,
    error,
    retries,
    failedAt: new Date().toISOString(),
  });
  await rdb.lpush(env.queue.deadQueue, payload);
  logger.warn(
    { jobId: job.id, url: job.url, retries },
    "job moved to dead letter queue",
  );
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
