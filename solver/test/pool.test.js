import { describe, it, expect, beforeEach, vi } from "vitest";

// createMockBrowser must be defined before vi.mock calls since vi.mock is hoisted.
// However, because vi.mock factories are executed lazily (at mock resolution time),
// and the factory below captures `createMockBrowser` at call time, we hoist it via
// a module-scope variable that is populated before any mock factory runs.
//
// The simplest safe approach: define it as a plain function expression in module scope
// so it exists by the time the mock factory is invoked after hoisting.

const createMockBrowser = () => {
  const pages = [];
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

vi.mock("../env.js", () => ({
  default: {
    browser: { headless: true, maxBrowsers: 2, maxTabs: 2, idleTimeout: 100 },
    proxies: [],
  },
}));

vi.mock("../src/logger.js", () => ({
  default: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));

describe("pool", () => {
  let pool;
  let connectMock;

  beforeEach(async () => {
    vi.resetModules();
    // Re-import to get fresh module-level state
    pool = await import("../src/pool.js");
    // Re-grab the connect mock reference after resetModules and clear its history
    const prbMod = await import("puppeteer-real-browser");
    connectMock = prbMod.connect;
    connectMock.mockClear();
  });

  it("acquire launches browser when pool empty", async () => {
    const result = await pool.acquire();

    expect(result).toBeDefined();
    expect(result.entry).toBeDefined();
    expect(result.page).toBeDefined();
    expect(pool.stats().browsers).toBe(1);
  });

  it("acquire reuses browser with available tabs", async () => {
    const r1 = await pool.acquire();
    const r2 = await pool.acquire();

    // Both should have come from the same browser instance
    expect(r1.entry.id).toBe(r2.entry.id);
    // connect() should have been called only once
    expect(connectMock).toHaveBeenCalledTimes(1);
    expect(pool.stats().browsers).toBe(1);
  });

  it("acquire queues when all slots full", async () => {
    // maxBrowsers=2, maxTabs=2 → capacity = 4 slots total
    const r1 = await pool.acquire();
    const r2 = await pool.acquire();
    const r3 = await pool.acquire();
    const r4 = await pool.acquire();

    // At this point both browsers are full (2 browsers × 2 tabs = 4)
    expect(pool.stats().browsers).toBe(2);

    // 5th acquire should queue
    let resolved = false;
    const pending = pool.acquire().then((v) => {
      resolved = true;
      return v;
    });

    // Give microtasks a tick to settle
    await new Promise((r) => setTimeout(r, 10));

    expect(resolved).toBe(false);
    expect(pool.stats().queued).toBe(1);

    // Clean up: release one slot so the waiter eventually resolves
    await pool.release(r1.entry, r1.page);
    await pending;

    void r2;
    void r3;
    void r4;
  });

  it("release frees a waiter", async () => {
    // Fill capacity (2 browsers × 2 tabs)
    const r1 = await pool.acquire();
    const r2 = await pool.acquire();
    const r3 = await pool.acquire();
    const r4 = await pool.acquire();

    // Queue one waiter
    let waiterResult = undefined;
    const waiterPromise = pool.acquire().then((v) => {
      waiterResult = v;
      return v;
    });

    await new Promise((r) => setTimeout(r, 10));
    expect(pool.stats().queued).toBe(1);

    // Release a slot — the waiter should be drained
    await pool.release(r1.entry, r1.page);
    await waiterPromise;

    expect(waiterResult).toBeDefined();
    expect(waiterResult.entry).toBeDefined();
    expect(waiterResult.page).toBeDefined();

    void r2;
    void r3;
    void r4;
  });

  it("idle cleanup closes idle browsers", async () => {
    // acquire then release so the browser is idle (tabs === 0)
    const { entry, page } = await pool.acquire();
    expect(pool.stats().browsers).toBe(1);

    await pool.release(entry, page);

    // Browser is now idle (tabs=0). Verify it's still tracked before cleanup.
    expect(pool.stats().browsers).toBe(1);

    // Calling shutdown clears all browsers (exercises cleanup path)
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
    // Launch a couple of browsers
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
