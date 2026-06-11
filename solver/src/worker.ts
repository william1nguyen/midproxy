import env from "../env";
import logger from "./logger";
import { isShuttingDown, stats } from "./pool";
import {
  ackJob,
  ensureConsumerGroup,
  incrementRetry,
  isActiveJob,
  pushDeadLetter,
  readJob,
  requeueJob,
  storeCookies,
} from "./redis";
import { solve } from "./solver";
import type { Job } from "./types";

const processJob = async (messageId: string, job: Job): Promise<void> => {
  const log = logger.child({ jobId: job.id, url: job.url, messageId });

  const domain = new URL(job.url).hostname;

  try {
    const active = await isActiveJob(domain, job.id);
    if (!active) {
      log.info("stale job, skipping");
      await ackJob(messageId);
      return;
    }

    log.info("solving");
    const result = await solve(job.url);
    await storeCookies(domain, result);
    await ackJob(messageId);
    log.info(
      { count: result.cookies.length, proxy: result.proxyURL },
      "solved",
    );
  } catch (err) {
    const error = err as Error;
    log.error({ err: error.message, stack: error.stack }, "solve failed");

    await ackJob(messageId);
    const retries = await incrementRetry(job.id);
    if (retries < env.queue.maxJobRetries) {
      log.info({ retries, max: env.queue.maxJobRetries }, "requeueing job");
      await requeueJob(job);
    } else {
      await pushDeadLetter(job, error.message, retries);
    }
  }
};

export const run = async (): Promise<void> => {
  await ensureConsumerGroup();
  logger.info("worker started, waiting for jobs");

  while (!isShuttingDown()) {
    try {
      const msg = await readJob();
      if (!msg) continue;
      processJob(msg.messageId, msg.job).catch((err: Error) => {
        logger.error({ err: err.message }, "unhandled job error");
      });
      logger.debug(stats(), "pool stats");
    } catch (err) {
      if (isShuttingDown()) break;
      logger.error({ err: (err as Error).message }, "queue error");
      await new Promise((r) => setTimeout(r, 1000));
    }
  }
};
