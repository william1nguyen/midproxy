import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const mockEntry = { id: 0, proxy: "http://proxy:1", tabs: 1, lastUsed: Date.now() };

vi.mock("../src/pool.js", () => ({
  acquire: vi.fn(),
  release: vi.fn(async () => {}),
}));

vi.mock("../env.js", () => ({
  default: {
    solver: { clearanceTimeout: 500, navigationTimeout: 5000 },
  },
}));

vi.mock("../src/logger.js", () => ({
  default: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));

const createMockPage = (cookies = [], userAgent = "TestUA") => ({
  goto: vi.fn(async () => {}),
  cookies: vi.fn(async () => cookies),
  evaluate: vi.fn(async () => userAgent),
  close: vi.fn(async () => {}),
  on: vi.fn(),
  off: vi.fn(),
});

describe("solver", () => {
  let solver;
  let acquireMock;
  let releaseMock;

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();
    solver = await import("../src/solver.js");
    const poolMod = await import("../src/pool.js");
    acquireMock = poolMod.acquire;
    releaseMock = poolMod.release;
    acquireMock.mockClear();
    releaseMock.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("solve returns cookies and userAgent on success", async () => {
    const cfCookie = { name: "cf_clearance", value: "abc123", domain: "example.com", path: "/" };
    const page = createMockPage([cfCookie], "TestUA/1.0");

    acquireMock.mockResolvedValue({ entry: mockEntry, page });

    const solvePromise = solver.solve("https://example.com");

    // Advance past the 5000ms setTimeout in solve
    await vi.advanceTimersByTimeAsync(5000);
    // Advance past any polling intervals in waitForClearance
    await vi.advanceTimersByTimeAsync(500);

    const result = await solvePromise;

    expect(result.userAgent).toBe("TestUA/1.0");
    expect(result.proxyURL).toBe("http://proxy:1");
    expect(result.cookies).toEqual([
      { name: "cf_clearance", value: "abc123", domain: "example.com", path: "/" },
    ]);
  });

  it("solve returns available cookies on timeout", async () => {
    const otherCookie = { name: "session", value: "xyz", domain: "example.com", path: "/" };
    const page = createMockPage([otherCookie], "TestUA/1.0");

    acquireMock.mockResolvedValue({ entry: mockEntry, page });

    const solvePromise = solver.solve("https://example.com");

    // Advance past the 5000ms setTimeout in solve
    await vi.advanceTimersByTimeAsync(5000);
    // Advance well past clearanceTimeout (500ms) to trigger timeout path
    await vi.advanceTimersByTimeAsync(3000);

    const result = await solvePromise;

    expect(result.userAgent).toBe("TestUA/1.0");
    expect(result.proxyURL).toBe("http://proxy:1");
    expect(result.cookies).toEqual([
      { name: "session", value: "xyz", domain: "example.com", path: "/" },
    ]);
  });

  it("solve throws when pool shutting down", async () => {
    acquireMock.mockResolvedValue(null);

    await expect(solver.solve("https://example.com")).rejects.toThrow("pool shutting down");
  });

  it("solve releases page on error", async () => {
    const page = createMockPage([], "TestUA/1.0");
    page.goto.mockRejectedValue(new Error("navigation failed"));

    acquireMock.mockResolvedValue({ entry: mockEntry, page });

    await expect(solver.solve("https://example.com")).rejects.toThrow("navigation failed");

    expect(releaseMock).toHaveBeenCalledWith(mockEntry, page);
  });
});
