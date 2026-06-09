import { connect } from "puppeteer-real-browser";
import env from "../env.js";
import logger from "./logger.js";
import { acquire, release, trackBrowser, untrackBrowser } from "./pool.js";

export const solve = async (targetURL, proxyURL) => {
  await acquire();

  const opts = {
    headless: env.browser.headless,
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

  let browser;
  try {
    ({ browser } = await connect(opts));
    trackBrowser(browser);
  } catch (err) {
    release();
    throw err;
  }

  try {
    const page = (await browser.pages())[0] || (await browser.newPage());

    await page.goto(targetURL, {
      waitUntil: "networkidle2",
      timeout: env.solver.navigationTimeout,
    });

    await new Promise((r) => setTimeout(r, 5000));

    const cookies = await waitForClearance(page);
    const userAgent = await page.evaluate(() => navigator.userAgent);
    logger.info({ cookies, userAgent }, "solve completed");

    return {
      userAgent,
      cookies: cookies.map((c) => ({
        name: c.name,
        value: c.value,
        domain: c.domain,
        path: c.path,
      })),
    };
  } finally {
    untrackBrowser(browser);
    await browser.close().catch(() => {});
    release();
  }
};

const waitForClearance = async (page) => {
  const deadline = Date.now() + env.solver.clearanceTimeout;

  const navigationPromise = new Promise((resolve) => {
    const handler = () => {
      page.off("framenavigated", handler);
      resolve();
    };
    page.on("framenavigated", handler);
  });

  while (Date.now() < deadline) {
    const cookies = await page.cookies();
    const cf = cookies.find((c) => c.name === "cf_clearance");

    if (cf) {
      logger.debug({ count: cookies.length }, "cf_clearance found");
      return cookies;
    }

    await Promise.race([
      navigationPromise,
      new Promise((r) => setTimeout(r, 2000)),
    ]);
  }

  logger.warn("cf_clearance timeout, returning available cookies");
  return await page.cookies();
};
