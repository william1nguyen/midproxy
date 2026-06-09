import "dotenv/config";

const env = {
  redis: {
    url: process.env.REDIS_URL || "redis://:redis@localhost:6379/1",
  },
  browser: {
    headless: false,
    maxBrowsers: parseInt(process.env.MAX_BROWSERS || "5", 10),
    maxTabs: parseInt(process.env.MAX_TABS || "3", 10),
    healthCheckInterval: parseInt(process.env.HEALTH_CHECK_INTERVAL || "30000", 10),
  },
  solver: {
    clearanceTimeout: parseInt(process.env.CLEARANCE_TIMEOUT || "30000", 10),
    navigationTimeout: parseInt(process.env.NAVIGATION_TIMEOUT || "60000", 10),
  },
  queue: {
    jobQueue: "queue:solve",
    cookieTTL: 1200,
  },
};

export default env;
