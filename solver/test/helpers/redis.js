import { GenericContainer } from "testcontainers";
import Redis from "ioredis";

let container;
let client;

export const setupRedis = async () => {
  container = await new GenericContainer("redis:7-alpine")
    .withExposedPorts(6379)
    .start();

  const host = container.getHost();
  const port = container.getMappedPort(6379);

  client = new Redis({ host, port, lazyConnect: false });
  await client.flushdb();
  return { client, host, port };
};

export const teardownRedis = async () => {
  if (client) client.disconnect();
  if (container) await container.stop();
};

export const getClient = () => client;
