import "dotenv/config";

const env = {
  redis: {
    url: process.env.REDIS_URL || "redis://:redis@localhost:6379/1",
  },
  browser: {
    headless: process.env.HEADLESS !== "false",
    maxTabs: parseInt(process.env.MAX_TABS || "3", 10),
  },
  solver: {
    clearanceTimeout: parseInt(process.env.CLEARANCE_TIMEOUT || "30000", 10),
    navigationTimeout: parseInt(process.env.NAVIGATION_TIMEOUT || "60000", 10),
  },
  queue: {
    jobQueue: "queue:solve",
    replyPrefix: "reply:",
    replyTTL: 120,
    cookiePrefix: "cookie:",
    cookieTTL: 1800,
  },
};

export default env;
