import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Job, SolveResult } from "../src/types";

let redisMock: {
  storeCookies: ReturnType<typeof vi.fn>;
  popJob: ReturnType<typeof vi.fn>;
  isActiveJob: ReturnType<typeof vi.fn>;
  shutdown: ReturnType<typeof vi.fn>;
};

let solverMock: {
  solve: ReturnType<typeof vi.fn>;
};

let poolMock: {
  isShuttingDown: ReturnType<typeof vi.fn>;
  stats: ReturnType<typeof vi.fn>;
};

vi.mock("../src/redis", () => ({
  get storeCookies() {
    return redisMock.storeCookies;
  },
  get popJob() {
    return redisMock.popJob;
  },
  get isActiveJob() {
    return redisMock.isActiveJob;
  },
  get shutdown() {
    return redisMock.shutdown;
  },
}));

vi.mock("../src/solver", () => ({
  get solve() {
    return solverMock.solve;
  },
}));

vi.mock("../src/pool", () => ({
  get isShuttingDown() {
    return poolMock.isShuttingDown;
  },
  get stats() {
    return poolMock.stats;
  },
}));

vi.mock("../src/logger", () => ({
  default: {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
    child: vi.fn(() => ({
      info: vi.fn(),
      warn: vi.fn(),
      error: vi.fn(),
      debug: vi.fn(),
    })),
  },
}));

const nextTick = () => new Promise<void>((r) => setTimeout(r, 0));

describe("worker", () => {
  let worker: typeof import("../src/worker");

  beforeEach(async () => {
    vi.resetModules();

    redisMock = {
      storeCookies: vi.fn(async () => {}),
      popJob: vi.fn(
        async (): Promise<Job> => ({ id: "j1", url: "http://test.com" }),
      ),
      isActiveJob: vi.fn(async () => true),
      shutdown: vi.fn(async () => {}),
    };

    solverMock = {
      solve: vi.fn(
        async (): Promise<SolveResult> => ({
          cookies: [
            { name: "cf", value: "abc", domain: "test.com", path: "/" },
          ],
          proxyURL: "http://proxy:8080",
          userAgent: "UA",
        }),
      ),
    };

    let callCount = 0;
    poolMock = {
      isShuttingDown: vi.fn(() => callCount++ > 0),
      stats: vi.fn(() => ({ browsers: 0, queued: 0, maxBrowsers: 2 })),
    };

    worker = await import("../src/worker");
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
    solverMock.solve = vi.fn(async () => {
      throw new Error("browser crash");
    });

    await expect(worker.run()).resolves.toBeUndefined();
    await nextTick();

    expect(solverMock.solve).toHaveBeenCalled();
    expect(redisMock.storeCookies).not.toHaveBeenCalled();
  });

  it("stops on shutdown immediately", async () => {
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
