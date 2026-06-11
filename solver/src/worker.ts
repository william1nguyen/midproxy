import logger from "./logger";
import { isShuttingDown, stats } from "./pool";
import { isActiveJob, popJob, storeCookies } from "./redis";
import { solve } from "./solver";
import type { Job } from "./types";

const processJob = async (job: Job): Promise<void> => {
  const log = logger.child({ jobId: job.id, url: job.url });

  const domain = new URL(job.url).hostname;

  try {
    const active = await isActiveJob(domain, job.id);
    if (!active) {
      log.info("stale job, skipping");
      return;
    }

    log.info("solving");
    const result = await solve(job.url);
    await storeCookies(domain, result);
    log.info(
      { count: result.cookies.length, proxy: result.proxyURL },
      "solved",
    );
  } catch (err) {
    log.error(
      { err: (err as Error).message, stack: (err as Error).stack },
      "solve failed",
    );
  }
};

export const run = async (): Promise<void> => {
  logger.info("worker started, waiting for jobs");

  while (!isShuttingDown()) {
    try {
      const job = await popJob();
      processJob(job).catch((err: Error) => {
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
