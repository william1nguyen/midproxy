import { connect } from "puppeteer-real-browser";
import env from "../env.js";
import logger from "./logger.js";

const entries = new Map();
const waiters = [];
let nextId = 0;
let launching = 0;
let shuttingDown = false;
let idleTimer;

const parseProxy = (proxyURL) => {
  if (!proxyURL) return undefined;
  const u = new URL(proxyURL);
  return { host: u.hostname, port: u.port, username: u.username, password: u.password };
};

const launchBrowser = async (proxy) => {
  const opts = { headless: env.browser.headless, turnstile: true };
  const parsed = parseProxy(proxy);
  if (parsed) opts.proxy = parsed;
  const { browser } = await connect(opts);
  return browser;
};

let proxyIndex = 0;

const pickProxy = () => {
  if (env.proxies.length === 0) return "";
  const proxy = env.proxies[proxyIndex % env.proxies.length];
  proxyIndex++;
  return proxy;
};

const findAvailable = () => {
  let best = null;
  for (const entry of entries.values()) {
    if (entry.tabs < env.browser.maxTabs) {
      if (!best || entry.tabs < best.tabs) best = entry;
    }
  }
  return best;
};

const drainWaiter = async (entry) => {
  if (waiters.length === 0 || entry.tabs >= env.browser.maxTabs) return;
  const resolve = waiters.shift();
  entry.tabs++;
  entry.lastUsed = Date.now();
  try {
    const page = await entry.browser.newPage();
    resolve({ entry, page });
  } catch (err) {
    entry.tabs--;
    logger.error({ err: err.message, id: entry.id }, "failed to create page for waiter");
    resolve(null);
  }
};

export const acquire = async () => {
  let entry = findAvailable();
  if (entry) {
    entry.tabs++;
    entry.lastUsed = Date.now();
    const page = await entry.browser.newPage();
    return { entry, page };
  }

  if (entries.size + launching < env.browser.maxBrowsers) {
    const proxy = pickProxy();
    launching++;
    let browser;
    try {
      browser = await launchBrowser(proxy);
    } catch (err) {
      launching--;
      throw err;
    }
    launching--;

    const id = nextId++;
    entry = { id, browser, proxy, tabs: 1, lastUsed: Date.now() };
    entries.set(id, entry);
    logger.info({ id, proxy: proxy || "direct" }, "browser launched");

    browser.on("disconnected", () => {
      logger.warn({ id, proxy: entry.proxy }, "browser disconnected");
      entries.delete(id);
    });

    const page = (await browser.pages())[0] || (await browser.newPage());
    return { entry, page };
  }

  return new Promise((resolve) => waiters.push(resolve));
};

export const release = async (entry, page) => {
  await page.close().catch(() => {});
  entry.tabs--;
  entry.lastUsed = Date.now();
  drainWaiter(entry);
};

const cleanupIdle = () => {
  const now = Date.now();
  for (const [id, entry] of entries) {
    if (entry.tabs === 0 && now - entry.lastUsed > env.browser.idleTimeout) {
      logger.info({ id, proxy: entry.proxy }, "closing idle browser");
      entry.browser.close().catch(() => {});
      entries.delete(id);
    }
  }
};

export const startIdleCleanup = () => {
  idleTimer = setInterval(cleanupIdle, 30000);
  idleTimer.unref();
};

export const stats = () => ({
  browsers: entries.size,
  launching,
  maxBrowsers: env.browser.maxBrowsers,
  queued: waiters.length,
  entries: [...entries.values()].map((e) => ({ id: e.id, proxy: e.proxy, tabs: e.tabs })),
});

export const isShuttingDown = () => shuttingDown;

export const shutdown = async () => {
  shuttingDown = true;
  if (idleTimer) clearInterval(idleTimer);
  for (const resolve of waiters) resolve(null);
  waiters.length = 0;
  for (const [, entry] of entries) {
    await entry.browser.close().catch(() => {});
  }
  entries.clear();
};
