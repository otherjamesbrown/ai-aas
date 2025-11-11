import Fastify from 'fastify';
import { loadConfig, startTelemetry, Registry, httpHandler, initDataAccess } from '@ai-aas/shared';

async function start() {
  const config = loadConfig();
  const telemetry = await startTelemetry({
    ...config.telemetry,
    serviceName: config.service.name,
    environment: process.env.DEPLOYMENT_ENVIRONMENT ?? 'development',
  });

  const app = Fastify({ logger: true });

  const registry = new Registry();
  registry.register('self', async () => {});

  let pool: ReturnType<typeof initDataAccess>['pool'] | undefined;
  if (config.database.dsn) {
    const dataAccess = initDataAccess(config.database);
    registry.register('database', dataAccess.probe);
    pool = dataAccess.pool;
  }

  app.get('/healthz', httpHandler(registry));
  app.get('/info', async () => ({
    service: config.service.name,
    version: '0.0.0',
  }));

  const close = async () => {
    await telemetry.shutdown();
    await app.close();
    await pool?.end?.();
  };

  process.on('SIGINT', () => close().then(() => process.exit(0)));
  process.on('SIGTERM', () => close().then(() => process.exit(0)));

  const address = { port: config.service.port, host: config.service.host };
  try {
    await app.listen(address);
    app.log.info(`service-template (ts) listening on ${address.host}:${address.port}`);
  } catch (err) {
    app.log.error({ err }, 'failed to start service');
    await close();
    process.exit(1);
  }
}

start();

