import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { BrowserEntry, Cookie, SolveResult } from "../src/types";

const mockEntry: BrowserEntry = {
  id: 0,
  browser: null,
  proxy: "http://proxy:1",
  tabs: 1,
  lastUsed: Date.now(),
};

vi.mock("../src/pool", () => ({
  acquire: vi.fn(),
  release: vi.fn(async () => {}),
}));

vi.mock("../env", () => ({
  default: {
    solver: { clearanceTimeout: 500, navigationTimeout: 5000 },
  },
}));

vi.mock("../src/logger", () => ({
  default: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));

const createMockPage = (cookies: Cookie[] = [], userAgent = "TestUA") => ({
  goto: vi.fn(async () => {}),
  cookies: vi.fn(async () => cookies),
  evaluate: vi.fn(async () => userAgent),
  close: vi.fn(async () => {}),
  on: vi.fn(),
  off: vi.fn(),
});

describe("solver", () => {
  let solver: typeof import("../src/solver");
  let acquireMock: ReturnType<typeof vi.fn>;
  let releaseMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();
    solver = await import("../src/solver");
    const poolMod = await import("../src/pool");
    acquireMock = poolMod.acquire as ReturnType<typeof vi.fn>;
    releaseMock = poolMod.release as ReturnType<typeof vi.fn>;
    acquireMock.mockClear();
    releaseMock.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("solve returns cookies and userAgent on success", async () => {
    const cfCookie: Cookie = {
      name: "cf_clearance",
      value: "abc123",
      domain: "example.com",
      path: "/",
    };
    const page = createMockPage([cfCookie], "TestUA/1.0");

    acquireMock.mockResolvedValue({ entry: mockEntry, page });

    const solvePromise = solver.solve("https://example.com");

    await vi.advanceTimersByTimeAsync(5000);
    await vi.advanceTimersByTimeAsync(500);

    const result: SolveResult = await solvePromise;

    expect(result.userAgent).toBe("TestUA/1.0");
    expect(result.proxyURL).toBe("http://proxy:1");
    expect(result.cookies).toEqual([
      {
        name: "cf_clearance",
        value: "abc123",
        domain: "example.com",
        path: "/",
      },
    ]);
  });

  it("solve returns available cookies on timeout", async () => {
    const otherCookie: Cookie = {
      name: "session",
      value: "xyz",
      domain: "example.com",
      path: "/",
    };
    const page = createMockPage([otherCookie], "TestUA/1.0");

    acquireMock.mockResolvedValue({ entry: mockEntry, page });

    const solvePromise = solver.solve("https://example.com");

    await vi.advanceTimersByTimeAsync(5000);
    await vi.advanceTimersByTimeAsync(3000);

    const result: SolveResult = await solvePromise;

    expect(result.userAgent).toBe("TestUA/1.0");
    expect(result.proxyURL).toBe("http://proxy:1");
    expect(result.cookies).toEqual([
      { name: "session", value: "xyz", domain: "example.com", path: "/" },
    ]);
  });

  it("solve throws when pool shutting down", async () => {
    acquireMock.mockResolvedValue(null);

    await expect(solver.solve("https://example.com")).rejects.toThrow(
      "pool shutting down",
    );
  });

  it("solve releases page on error", async () => {
    const page = createMockPage([], "TestUA/1.0");
    page.goto.mockRejectedValue(new Error("navigation failed"));

    acquireMock.mockResolvedValue({ entry: mockEntry, page });

    await expect(solver.solve("https://example.com")).rejects.toThrow(
      "navigation failed",
    );

    expect(releaseMock).toHaveBeenCalledWith(mockEntry, page);
  });
});
