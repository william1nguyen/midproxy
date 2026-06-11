// solver/test/redis.test.js
import { describe, it, expect, beforeAll, afterAll, beforeEach, vi } from "vitest";
import { setupRedis, teardownRedis, getClient } from "./helpers/redis.js";

let redisModule;

describe("redis operations", () => {
  beforeAll(async () => {
    const { host, port } = await setupRedis();
    process.env.REDIS_URL = `redis://${host}:${port}/0`;
    vi.resetModules();
    redisModule = await import("../src/redis.js");
  });

  afterAll(async () => {
    await redisModule.shutdown();
    await teardownRedis();
  });

  beforeEach(async () => {
    await getClient().flushdb();
  });

  it("storeCookies stores one item with TTL", async () => {
    const result = { userAgent: "TestUA", cookies: [{ name: "cf", value: "abc" }], proxyURL: "" };
    await redisModule.storeCookies("test.com", result);

    const rdb = getClient();
    const len = await rdb.llen("cookies:test.com");
    expect(len).toBe(1);

    const ttl = await rdb.ttl("cookies:test.com");
    expect(ttl).toBeGreaterThan(0);
  });

  it("storeCookies multiple times appends", async () => {
    const result = { userAgent: "UA", cookies: [], proxyURL: "" };
    await redisModule.storeCookies("multi.com", result);
    await redisModule.storeCookies("multi.com", result);
    await redisModule.storeCookies("multi.com", result);

    const len = await getClient().llen("cookies:multi.com");
    expect(len).toBe(3);
  });

  it("popJob returns pushed job", async () => {
    const rdb = getClient();
    const job = JSON.stringify({ id: "test-123", url: "http://example.com" });
    await rdb.lpush("queue:solve", job);

    const result = await redisModule.popJob();
    expect(result.id).toBe("test-123");
    expect(result.url).toBe("http://example.com");
  });

  it("isActiveJob matches current lock", async () => {
    const rdb = getClient();
    await rdb.set("solving:active.com", "job-abc");

    const active = await redisModule.isActiveJob("active.com", "job-abc");
    expect(active).toBe(true);

    const stale = await redisModule.isActiveJob("active.com", "job-old");
    expect(stale).toBe(false);
  });
});
