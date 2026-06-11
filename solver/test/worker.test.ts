import { beforeEach, describe, expect, it, vi } from "vitest";
import type { StreamJob } from "../src/redis";
import type { Job, SolveResult } from "../src/types";

let redisMock: {
  ensureConsumerGroup: ReturnType<typeof vi.fn>;
  readJob: ReturnType<typeof vi.fn>;
  ackJob: ReturnType<typeof vi.fn>;
  storeCookies: ReturnType<typeof vi.fn>;
  isActiveJob: ReturnType<typeof vi.fn>;
  incrementRetry: ReturnType<typeof vi.fn>;
  requeueJob: ReturnType<typeof vi.fn>;
  pushDeadLetter: ReturnType<typeof vi.fn>;
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
  get ensureConsumerGroup() {
    return redisMock.ensureConsumerGroup;
  },
  get readJob() {
    return redisMock.readJob;
  },
  get ackJob() {
    return redisMock.ackJob;
  },
  get storeCookies() {
    return redisMock.storeCookies;
  },
  get isActiveJob() {
    return redisMock.isActiveJob;
  },
  get incrementRetry() {
    return redisMock.incrementRetry;
  },
  get requeueJob() {
    return redisMock.requeueJob;
  },
  get pushDeadLetter() {
    return redisMock.pushDeadLetter;
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

vi.mock("../env", () => ({
  default: {
    queue: { deadQueue: "queue:dead", cookieTTL: 1200, maxJobRetries: 3 },
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

const makeStreamJob = (job: Job): StreamJob => ({
  messageId: "1234567890-0",
  job,
});

describe("worker", () => {
  let worker: typeof import("../src/worker");

  beforeEach(async () => {
    vi.resetModules();

    const defaultJob: StreamJob = makeStreamJob({
      id: "j1",
      url: "http://test.com",
    });

    redisMock = {
      ensureConsumerGroup: vi.fn(async () => {}),
      readJob: vi.fn(async (): Promise<StreamJob> => defaultJob),
      ackJob: vi.fn(async () => {}),
      storeCookies: vi.fn(async () => {}),
      isActiveJob: vi.fn(async () => true),
      incrementRetry: vi.fn(async () => 1),
      requeueJob: vi.fn(async () => {}),
      pushDeadLetter: vi.fn(async () => {}),
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
    expect(redisMock.ackJob).toHaveBeenCalledWith("1234567890-0");
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
    expect(redisMock.ackJob).toHaveBeenCalled();
    expect(redisMock.incrementRetry).toHaveBeenCalled();
  });

  it("stops on shutdown immediately", async () => {
    poolMock.isShuttingDown = vi.fn(() => true);

    await worker.run();

    expect(redisMock.readJob).not.toHaveBeenCalled();
  });

  it("skips stale job", async () => {
    redisMock.isActiveJob = vi.fn(async () => false);

    await worker.run();
    await nextTick();

    expect(redisMock.isActiveJob).toHaveBeenCalled();
    expect(redisMock.ackJob).toHaveBeenCalled();
    expect(solverMock.solve).not.toHaveBeenCalled();
  });
});
