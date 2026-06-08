import env from "../env.js";
import logger from "./logger.js";
import { openPage } from "./pool.js";

export const solve = async (targetURL, proxyURL) => {
  const { page, close } = await openPage(proxyURL);

  try {
    await page.goto(targetURL, {
      waitUntil: "domcontentloaded",
      timeout: env.solver.navigationTimeout,
    });

    const cookies = await waitForClearance(page);

    return cookies.map((c) => ({
      name: c.name,
      value: c.value,
      domain: c.domain,
      path: c.path,
    }));
  } finally {
    await close();
  }
};

const waitForClearance = async (page) => {
  const deadline = Date.now() + env.solver.clearanceTimeout;

  while (Date.now() < deadline) {
    const cookies = await page.cookies();

    if (cookies.find((c) => c.name === "cf_clearance")) {
      logger.debug({ count: cookies.length }, "cf_clearance found");
      return cookies;
    }

    await new Promise((r) => setTimeout(r, 500));
  }

  logger.warn("cf_clearance timeout, returning available cookies");
  return await page.cookies();
};
