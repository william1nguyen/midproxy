import env from "./env";
import logger from "./src/logger";
import { shutdown as shutdownPool, startIdleCleanup } from "./src/pool";
import { shutdown as shutdownRedis } from "./src/redis";
import { run } from "./src/worker";

let shuttingDown = false;

const shutdown = async (): Promise<void> => {
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
