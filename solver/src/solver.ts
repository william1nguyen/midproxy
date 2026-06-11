import env from "../env";
import logger from "./logger";
import { acquire, release } from "./pool";
import type { SolveResult } from "./types";

export const solve = async (targetURL: string): Promise<SolveResult> => {
  const result = await acquire();
  if (!result) throw new Error("pool shutting down");

  const { entry, page } = result;

  try {
    await page.goto(targetURL, {
      waitUntil: "networkidle2",
      timeout: env.solver.navigationTimeout,
    });

    await new Promise((r) => setTimeout(r, 5000));

    const cookies = await waitForClearance(page);
    const userAgent: string = await page.evaluate(() => navigator.userAgent);

    logger.info(
      { cookies: cookies.length, proxy: entry.proxy },
      "solve completed",
    );

    return {
      userAgent,
      proxyURL: entry.proxy,
      cookies: cookies.map((c: any) => ({
        name: c.name as string,
        value: c.value as string,
        domain: c.domain as string,
        path: c.path as string,
      })),
    };
  } finally {
    await release(entry, page);
  }
};

const waitForClearance = async (page: any): Promise<any[]> => {
  const deadline = Date.now() + env.solver.clearanceTimeout;

  const navigationPromise = new Promise<void>((resolve) => {
    const handler = () => {
      page.off("framenavigated", handler);
      resolve();
    };
    page.on("framenavigated", handler);
  });

  while (Date.now() < deadline) {
    const cookies = await page.cookies();
    const cf = cookies.find((c: any) => c.name === "cf_clearance");

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
