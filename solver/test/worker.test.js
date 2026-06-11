import { describe, it, expect, beforeEach, vi } from "vitest";

// Module-level mocks — vi.mock is hoisted so these factories run before imports.
// We use module-scope variables that get re-assigned in beforeEach so each test
// can inject its own behaviour.

let redisMock;
let solverMock;
let poolMock;
let loggerMock;

vi.mock("../src/redis.js", () => ({
  get storeCookies() { return redisMock.storeCookies; },
  get popJob()       { return redisMock.popJob; },
  get isActiveJob()  { return redisMock.isActiveJob; },
  get shutdown()     { return redisMock.shutdown; },
}));

vi.mock("../src/solver.js", () => ({
  get solve() { return solverMock.solve; },
}));

vi.mock("../src/pool.js", () => ({
  get isShuttingDown() { return poolMock.isShuttingDown; },
  get stats()          { return poolMock.stats; },
}));

vi.mock("../src/logger.js", () => ({
  default: {
    info:  vi.fn(),
    warn:  vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
    child: vi.fn(() => ({
      info:  vi.fn(),
      warn:  vi.fn(),
      error: vi.fn(),
      debug: vi.fn(),
    })),
  },
}));

// Helper: wait one macrotask so fire-and-forget processJob() can settle.
const nextTick = () => new Promise((r) => setTimeout(r, 0));

describe("worker", () => {
  let worker;

  beforeEach(async () => {
    // Reset call counters etc. but keep mock shapes.
    vi.resetModules();

    // Build fresh mock objects for every test.
    redisMock = {
      storeCookies: vi.fn(async () => {}),
      popJob:       vi.fn(async () => ({ id: "j1", url: "http://test.com" })),
      isActiveJob:  vi.fn(async () => true),
      shutdown:     vi.fn(async () => {}),
    };

    solverMock = {
      solve: vi.fn(async () => ({
        cookies: [{ name: "cf", value: "abc" }],
        proxyURL: "http://proxy:8080",
        userAgent: "UA",
      })),
    };

    let callCount = 0;
    poolMock = {
      // Returns false on first call (enter loop), true on second (exit loop).
      isShuttingDown: vi.fn(() => callCount++ > 0),
      stats: vi.fn(() => ({ browsers: 0, queued: 0, maxBrowsers: 2 })),
    };

    worker = await import("../src/worker.js");
  });

  it("processes job successfully", async () => {
    await worker.run();
    await nextTick();

    expect(solverMock.solve).toHaveBeenCalledWith("http://test.com");
    expect(redisMock.storeCookies).toHaveBeenCalledWith(
      "test.com",
      expect.objectContaining({ cookies: expect.any(Array) }),
    );
  });

  it("continues on solve error", async () => {
    solverMock.solve = vi.fn(async () => { throw new Error("browser crash"); });

    // Should not throw despite solve failing.
    await expect(worker.run()).resolves.toBeUndefined();
    await nextTick();

    expect(solverMock.solve).toHaveBeenCalled();
    expect(redisMock.storeCookies).not.toHaveBeenCalled();
  });

  it("stops on shutdown immediately", async () => {
    // isShuttingDown returns true on the very first check → loop never enters.
    poolMock.isShuttingDown = vi.fn(() => true);

    await worker.run();

    expect(redisMock.popJob).not.toHaveBeenCalled();
  });

  it("skips stale job", async () => {
    redisMock.isActiveJob = vi.fn(async () => false);

    await worker.run();
    await nextTick();

    expect(redisMock.isActiveJob).toHaveBeenCalled();
    expect(solverMock.solve).not.toHaveBeenCalled();
  });
});
