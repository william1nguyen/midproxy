import { connect } from "puppeteer-real-browser";
import env from "../env.js";
import logger from "./logger.js";

// --- BrowserEntry ---

class BrowserEntry {
  constructor(browser, proxyURL) {
    this.browser = browser;
    this.proxyURL = proxyURL;
    this.activeTabs = 0;
    this.lastUsed = Date.now();
    this.healthy = true;
    this.tabQueue = [];
  }

  get key() {
    return this.proxyURL || "__direct__";
  }

  get idle() {
    return this.activeTabs === 0;
  }

  get full() {
    return this.activeTabs >= env.browser.maxTabs;
  }

  touch() {
    this.lastUsed = Date.now();
  }
}

// --- Pool state ---

const entries = new Map();
const globalWaiters = [];
let healthCheckTimer = null;

// --- Internal helpers ---

const launchBrowser = async (proxyURL) => {
  const key = proxyURL || "__direct__";
  logger.info({ proxy: key }, "launching browser");

  const opts = {
    headless: env.browser.headless,
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
    turnstile: true,
  };

  if (proxyURL) {
    const u = new URL(proxyURL);
    opts.proxy = {
      host: u.hostname,
      port: u.port,
      username: u.username,
      password: u.password,
    };
  }

  const { browser, page } = await connect(opts);
  await page.close();

  const entry = new BrowserEntry(browser, proxyURL);
  entries.set(key, entry);

  logger.info({ proxy: key, poolSize: entries.size }, "browser launched");
  return entry;
};

const closeBrowser = async (entry) => {
  const key = entry.key;
  logger.info({ proxy: key }, "closing browser");
  entry.healthy = false;
  entries.delete(key);
  await entry.browser.close().catch(() => {});

  if (globalWaiters.length > 0) {
    const resolve = globalWaiters.shift();
    resolve();
  }
};

const findLRUIdle = () => {
  let oldest = null;
  for (const entry of entries.values()) {
    if (!entry.idle) continue;
    if (!oldest || entry.lastUsed < oldest.lastUsed) {
      oldest = entry;
    }
  }
  return oldest;
};

// --- Public API ---

export const checkout = async (proxyURL) => {
  const key = proxyURL || "__direct__";

  if (entries.has(key)) {
    const entry = entries.get(key);

    if (!entry.full) {
      entry.activeTabs++;
      entry.touch();
      const page = await entry.browser.newPage();
      return { page, release: () => release(key) };
    }

    await new Promise((resolve) => entry.tabQueue.push(resolve));
    entry.activeTabs++;
    entry.touch();
    const page = await entry.browser.newPage();
    return { page, release: () => release(key) };
  }

  if (entries.size >= env.browser.maxBrowsers) {
    const lru = findLRUIdle();
    if (lru) {
      await closeBrowser(lru);
    } else {
      await new Promise((resolve) => globalWaiters.push(resolve));
      return checkout(proxyURL);
    }
  }

  const entry = await launchBrowser(proxyURL);
  entry.activeTabs++;
  entry.touch();
  const page = await entry.browser.newPage();
  return { page, release: () => release(key) };
};

const release = async (key) => {
  const entry = entries.get(key);
  if (!entry) return;

  entry.activeTabs = Math.max(0, entry.activeTabs - 1);
  entry.touch();

  if (entry.tabQueue.length > 0) {
    const resolve = entry.tabQueue.shift();
    resolve();
    return;
  }

  if (entry.idle && globalWaiters.length > 0) {
    const resolve = globalWaiters.shift();
    resolve();
  }
};

// --- Health check ---

const runHealthCheck = async () => {
  for (const [key, entry] of entries) {
    try {
      await entry.browser.pages();
    } catch {
      logger.warn({ proxy: key }, "browser health check failed, removing");
      await closeBrowser(entry);

      while (entry.tabQueue.length > 0) {
        const resolve = entry.tabQueue.shift();
        resolve();
      }
    }
  }
};

export const startHealthCheck = () => {
  if (healthCheckTimer) return;
  healthCheckTimer = setInterval(runHealthCheck, env.browser.healthCheckInterval);
  logger.info({ intervalMs: env.browser.healthCheckInterval }, "health check started");
};

export const stopHealthCheck = () => {
  if (healthCheckTimer) {
    clearInterval(healthCheckTimer);
    healthCheckTimer = null;
  }
};

// --- Shutdown ---

export const shutdown = async () => {
  stopHealthCheck();
  for (const [key, entry] of entries) {
    logger.info({ proxy: key }, "closing browser");
    await entry.browser.close().catch(() => {});
  }
  entries.clear();
  globalWaiters.length = 0;
};

// --- Stats (for logging/debugging) ---

export const stats = () => ({
  browsers: entries.size,
  maxBrowsers: env.browser.maxBrowsers,
  entries: [...entries.values()].map((e) => ({
    proxy: e.key,
    activeTabs: e.activeTabs,
    healthy: e.healthy,
    idle: e.idle,
    lastUsed: e.lastUsed,
  })),
});
