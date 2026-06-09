import env from "./env.js";
import logger from "./src/logger.js";
import { shutdown as shutdownRedis } from "./src/redis.js";
import { shutdown as shutdownPool, startHealthCheck } from "./src/pool.js";
import { run } from "./src/worker.js";

const shutdown = async () => {
  logger.info("shutting down");
  await shutdownPool();
  await shutdownRedis();
  process.exit(0);
};

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);

logger.info(
  {
    headless: env.browser.headless,
    maxBrowsers: env.browser.maxBrowsers,
    maxTabs: env.browser.maxTabs,
    maxConcurrency: env.browser.maxBrowsers * env.browser.maxTabs,
    healthCheckInterval: env.browser.healthCheckInterval,
  },
  "solver service starting",
);

startHealthCheck();
run();
