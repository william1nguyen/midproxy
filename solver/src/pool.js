import env from "../env.js";
import logger from "./logger.js";

let active = 0;
const queue = [];
const browsers = new Set();

export const acquire = async () => {
  if (active < env.browser.maxBrowsers) {
    active++;
    return;
  }

  await new Promise((resolve) => queue.push(resolve));
  active++;
};

export const release = () => {
  active--;
  if (queue.length > 0) {
    queue.shift()();
  }
};

export const trackBrowser = (browser) => {
  browsers.add(browser);
};

export const untrackBrowser = (browser) => {
  browsers.delete(browser);
};

export const stats = () => ({
  active,
  maxBrowsers: env.browser.maxBrowsers,
  queued: queue.length,
  tracked: browsers.size,
});

let _shuttingDown = false;

export const isShuttingDown = () => _shuttingDown;

export const shutdown = async () => {
  _shuttingDown = true;

  while (queue.length > 0) {
    queue.shift()();
  }

  const closePromises = [...browsers].map((b) =>
    b.close().catch((err) => logger.warn({ err: err.message }, "error closing browser during shutdown"))
  );
  await Promise.allSettled(closePromises);

  browsers.clear();
  active = 0;
  logger.info("all browsers closed");
};
