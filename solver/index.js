import env from "./env.js";
import logger from "./src/logger.js";
import { shutdown as shutdownPool, startIdleCleanup } from "./src/pool.js";
import { shutdown as shutdownRedis } from "./src/redis.js";
import { run } from "./src/worker.js";

let shuttingDown = false;

const shutdown = async () => {
  if (shuttingDown) return;
  shuttingDown = true;

  setTimeout(() => process.exit(1), 5000);

  logger.info("shutting down...");

  await shutdownPool();
  await shutdownRedis();

  logger.info("shutdown complete");
  process.exit(0);
};

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);

logger.info(
  {
    headless: env.browser.headless,
    maxBrowsers: env.browser.maxBrowsers,
    maxTabs: env.browser.maxTabs,
    proxies: env.proxies.length,
  },
  "solver service starting",
);

startIdleCleanup();
run();
