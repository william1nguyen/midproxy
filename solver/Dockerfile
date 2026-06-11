# Stage 1: Build
FROM node:22-slim AS builder
RUN corepack enable && corepack prepare pnpm@10.30.3 --activate
WORKDIR /app
COPY package.json pnpm-lock.yaml tsup.config.ts tsconfig.json ./
COPY env.ts index.ts ./
COPY src/ ./src/
RUN PUPPETEER_SKIP_DOWNLOAD=true pnpm install --frozen-lockfile
RUN pnpm build

# Stage 2: Production deps only
FROM node:22-slim AS deps
RUN corepack enable && corepack prepare pnpm@10.30.3 --activate
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN PUPPETEER_SKIP_DOWNLOAD=true pnpm install --frozen-lockfile --prod

# Stage 3: Runtime
FROM node:22-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    fonts-liberation \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libgbm1 \
    libnspr4 \
    libnss3 \
    libx11-xcb1 \
    libxcomposite1 \
    libxdamage1 \
    libxrandr2 \
    xdg-utils \
    && rm -rf /var/lib/apt/lists/*

ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY --from=builder /app/dist ./dist
CMD ["node", "dist/index.js"]
