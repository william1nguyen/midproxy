export interface Cookie {
  name: string;
  value: string;
  domain: string;
  path: string;
}

export interface SolveResult {
  userAgent: string;
  proxyURL: string;
  cookies: Cookie[];
}

export interface Job {
  id: string;
  url: string;
}

export interface BrowserEntry {
  id: number;
  browser: any;
  proxy: string;
  tabs: number;
  lastUsed: number;
}

export interface AcquireResult {
  entry: BrowserEntry;
  page: any;
}

export interface EnvConfig {
  redis: { url: string };
  browser: {
    headless: boolean;
    maxBrowsers: number;
    maxTabs: number;
    idleTimeout: number;
  };
  solver: {
    clearanceTimeout: number;
    navigationTimeout: number;
  };
  queue: {
    deadQueue: string;
    cookieTTL: number;
    maxJobRetries: number;
  };
  proxies: string[];
}
