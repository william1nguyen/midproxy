import logger from "./logger.js";
import { storeCookies, popJob } from "./redis.js";
import { solve } from "./solver.js";
import { stats, isShuttingDown } from "./pool.js";

const processJob = async (job) => {
  const log = logger.child({ jobId: job.id, url: job.url });

  try {
    log.info("solving");
    const result = await solve(job.url, job.proxy);
    log.info({ cookies: result.cookies.length, ua: result.userAgent?.slice(0, 30) }, "solve returned");
    const domain = new URL(job.url).hostname;
    result.proxyURL = job.proxy || "";
    await storeCookies(domain, result);
    log.info({ count: result.cookies.length }, "solved");
  } catch (err) {
    log.error({ err: err.message, stack: err.stack }, "solve failed");
  }
};

export const run = async () => {
  logger.info("worker started, waiting for jobs");

  while (!isShuttingDown()) {
    try {
      const job = await popJob();
      processJob(job).catch((err) => {
        logger.error({ err: err.message }, "unhandled job error");
      });
      logger.debug(stats(), "pool stats");
    } catch (err) {
      if (isShuttingDown()) break;
      logger.error({ err: err.message }, "queue error");
      await new Promise((r) => setTimeout(r, 1000));
    }
  }
};
