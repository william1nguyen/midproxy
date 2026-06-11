import {
  afterAll,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import type { SolveResult } from "../src/types";
import { getClient, setupRedis, teardownRedis } from "./helpers/redis";

let redisModule: typeof import("../src/redis");

describe("redis operations", () => {
  beforeAll(async () => {
    const { host, port } = await setupRedis();
    process.env.REDIS_URL = `redis://${host}:${port}/0`;
    vi.resetModules();
    redisModule = await import("../src/redis");
  });

  afterAll(async () => {
    await redisModule.shutdown();
    await teardownRedis();
  });

  beforeEach(async () => {
    await getClient().flushdb();
  });

  it("storeCookies stores one item with TTL", async () => {
    const result: SolveResult = {
      userAgent: "TestUA",
      cookies: [{ name: "cf", value: "abc", domain: "test.com", path: "/" }],
      proxyURL: "",
    };
    await redisModule.storeCookies("test.com", result);

    const rdb = getClient();
    const len = await rdb.llen("cookies:test.com");
    expect(len).toBe(1);

    const ttl = await rdb.ttl("cookies:test.com");
    expect(ttl).toBeGreaterThan(0);
  });

  it("storeCookies multiple times appends", async () => {
    const result: SolveResult = { userAgent: "UA", cookies: [], proxyURL: "" };
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
