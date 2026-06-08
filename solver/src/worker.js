import logger from "./logger.js";
import { storeCookies, pushReply, popJob } from "./redis.js";
import { solve } from "./solver.js";

const processJob = async (job) => {
  const log = logger.child({ jobId: job.id, url: job.url });

  try {
    log.info("solving");
    const cookies = await solve(job.url, job.proxy);

    const domain = new URL(job.url).hostname;
    await storeCookies(domain, cookies);
    await pushReply(job.id, { cookies });

    log.info({ count: cookies.length }, "solved");
  } catch (err) {
    log.error({ err: err.message }, "solve failed");
    await pushReply(job.id, { error: err.message, cookies: [] });
  }
};

export const run = async () => {
  logger.info("worker started, waiting for jobs");

  while (true) {
    try {
      const job = await popJob();
      processJob(job);
    } catch (err) {
      logger.error({ err: err.message }, "queue error");
      await new Promise((r) => setTimeout(r, 1000));
    }
  }
};
