import Redis from "ioredis";
import { GenericContainer, type StartedTestContainer } from "testcontainers";

let container: StartedTestContainer | undefined;
let client: Redis | undefined;

export const setupRedis = async (): Promise<{
  client: Redis;
  host: string;
  port: number;
}> => {
  container = await new GenericContainer("redis:7-alpine")
    .withExposedPorts(6379)
    .start();

  const host = container.getHost();
  const port = container.getMappedPort(6379);

  client = new Redis({ host, port, lazyConnect: false });
  await client.flushdb();
  return { client, host, port };
};

export const teardownRedis = async (): Promise<void> => {
  if (client) client.disconnect();
  if (container) await container.stop();
};

export const getClient = (): Redis => client!;
