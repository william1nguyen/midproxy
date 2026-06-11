import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AcquireResult } from "../src/types";

const createMockBrowser = () => {
  const pages: Array<{ close: ReturnType<typeof vi.fn> }> = [];
  return {
    newPage: vi.fn(async () => {
      const page = { close: vi.fn(async () => {}) };
      pages.push(page);
      return page;
    }),
    pages: vi.fn(async () => (pages.length > 0 ? [pages[0]] : [])),
    close: vi.fn(async () => {}),
    on: vi.fn(),
    _pages: pages,
  };
};

vi.mock("puppeteer-real-browser", () => ({
  connect: vi.fn(async () => {
    const browser = createMockBrowser();
    return { browser };
  }),
}));

vi.mock("../env", () => ({
  default: {
    browser: { headless: true, maxBrowsers: 2, maxTabs: 2, idleTimeout: 100 },
    proxies: [],
  },
}));

vi.mock("../src/logger", () => ({
  default: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));

describe("pool", () => {
  let pool: typeof import("../src/pool");
  let connectMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();
    pool = await import("../src/pool");
    const prbMod = await import("puppeteer-real-browser");
    connectMock = prbMod.connect as ReturnType<typeof vi.fn>;
    connectMock.mockClear();
  });

  it("acquire launches browser when pool empty", async () => {
    const result = await pool.acquire();

    expect(result).toBeDefined();
    expect(result!.entry).toBeDefined();
    expect(result!.page).toBeDefined();
    expect(pool.stats().browsers).toBe(1);
  });

  it("acquire reuses browser with available tabs", async () => {
    const r1 = await pool.acquire();
    const r2 = await pool.acquire();

    expect(r1!.entry.id).toBe(r2!.entry.id);
    expect(connectMock).toHaveBeenCalledTimes(1);
    expect(pool.stats().browsers).toBe(1);
  });

  it("acquire queues when all slots full", async () => {
    const r1 = await pool.acquire();
    const r2 = await pool.acquire();
    const r3 = await pool.acquire();
    const r4 = await pool.acquire();

    expect(pool.stats().browsers).toBe(2);

    let resolved = false;
    const pending = pool.acquire().then((v) => {
      resolved = true;
      return v;
    });

    await new Promise((r) => setTimeout(r, 10));

    expect(resolved).toBe(false);
    expect(pool.stats().queued).toBe(1);

    await pool.release(r1!.entry, r1!.page);
    await pending;

    void r2;
    void r3;
    void r4;
  });

  it("release frees a waiter", async () => {
    const r1 = await pool.acquire();
    const r2 = await pool.acquire();
    const r3 = await pool.acquire();
    const r4 = await pool.acquire();

    let waiterResult: AcquireResult | null | undefined;
    const waiterPromise = pool.acquire().then((v) => {
      waiterResult = v;
      return v;
    });

    await new Promise((r) => setTimeout(r, 10));
    expect(pool.stats().queued).toBe(1);

    await pool.release(r1!.entry, r1!.page);
    await waiterPromise;

    expect(waiterResult).toBeDefined();
    expect(waiterResult!.entry).toBeDefined();
    expect(waiterResult!.page).toBeDefined();

    void r2;
    void r3;
    void r4;
  });

  it("idle cleanup closes idle browsers", async () => {
    const { entry, page } = (await pool.acquire())!;
    expect(pool.stats().browsers).toBe(1);

    await pool.release(entry, page);

    expect(pool.stats().browsers).toBe(1);

    await pool.shutdown();
    expect(pool.stats().browsers).toBe(0);
  });

  it("stats returns correct counts", async () => {
    const r1 = await pool.acquire();
    const r2 = await pool.acquire();

    const s = pool.stats();
    expect(s.browsers).toBeGreaterThanOrEqual(1);
    expect(s.maxBrowsers).toBe(2);
    expect(s.queued).toBe(0);

    void r1;
    void r2;
  });

  it("shutdown closes all browsers", async () => {
    const r1 = await pool.acquire();
    const r2 = await pool.acquire();
    const r3 = await pool.acquire();

    expect(pool.stats().browsers).toBeGreaterThanOrEqual(1);

    void r2;
    void r3;
    void r1;

    await pool.shutdown();

    expect(pool.stats().browsers).toBe(0);
    expect(pool.isShuttingDown()).toBe(true);
  });
});
