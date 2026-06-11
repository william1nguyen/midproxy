import "dotenv/config";
import type { EnvConfig } from "./src/types";

const env: EnvConfig = {
  redis: {
    url: process.env.REDIS_URL || "redis://:redis@localhost:6379/1",
  },
  browser: {
    headless: process.env.HEADLESS === "true",
    maxBrowsers: parseInt(process.env.MAX_BROWSERS || "3", 10),
    maxTabs: parseInt(process.env.MAX_TABS || "3", 10),
    idleTimeout: parseInt(process.env.IDLE_TIMEOUT || "300000", 10),
  },
  solver: {
    clearanceTimeout: parseInt(process.env.CLEARANCE_TIMEOUT || "30000", 10),
    navigationTimeout: parseInt(process.env.NAVIGATION_TIMEOUT || "60000", 10),
  },
  queue: {
    jobQueue: "queue:solve",
    cookieTTL: 1200,
  },
  proxies: (process.env.PROXY_LIST || "").split(",").filter(Boolean),
};

export default env;
