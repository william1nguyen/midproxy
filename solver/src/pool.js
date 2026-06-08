import { connect } from "puppeteer-real-browser";
import env from "../env.js";
import logger from "./logger.js";

const browsers = new Map();
const semaphores = new Map();

const getSemaphore = (key) => {
  if (!semaphores.has(key)) {
    semaphores.set(key, { active: 0, queue: [] });
  }
  return semaphores.get(key);
};

const acquireTab = (proxyURL) => {
  const key = proxyURL || "__direct__";
  const sem = getSemaphore(key);

  if (sem.active < env.browser.maxTabs) {
    sem.active++;
    return Promise.resolve();
  }

  return new Promise((resolve) => sem.queue.push(resolve));
};

const releaseTab = (proxyURL) => {
  const key = proxyURL || "__direct__";
  const sem = getSemaphore(key);

  if (sem.queue.length > 0) {
    sem.queue.shift()();
  } else {
    sem.active--;
  }
};

const getBrowser = async (proxyURL) => {
  const key = proxyURL || "__direct__";

  if (browsers.has(key)) return browsers.get(key);

  logger.info({ proxy: key }, "launching browser");

  const opts = {
    headless: env.browser.headless,
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
    turnstile: true,
  };

  if (proxyURL) {
    opts.proxy = { host: proxyURL };
  }

  const { browser, page } = await connect(opts);
  await page.close();

  browsers.set(key, browser);
  return browser;
};

export const openPage = async (proxyURL) => {
  await acquireTab(proxyURL);

  const browser = await getBrowser(proxyURL);
  const page = await browser.newPage();

  return {
    page,
    close: async () => {
      await page.close().catch(() => {});
      releaseTab(proxyURL);
    },
  };
};

export const shutdown = async () => {
  for (const [key, browser] of browsers) {
    logger.info({ proxy: key }, "closing browser");
    await browser.close().catch(() => {});
  }
  browsers.clear();
  semaphores.clear();
};
