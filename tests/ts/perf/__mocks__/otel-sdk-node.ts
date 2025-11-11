export class NodeSDK {
  public config: unknown;

  constructor(config: unknown) {
    this.config = config;
  }

  async start() {
    const handler = (globalThis as any).__otelNodeStartMock;
    if (typeof handler === 'function') {
      handler(this.config);
    }
  }

  async shutdown() {
    const handler = (globalThis as any).__otelNodeShutdownMock;
    if (typeof handler === 'function') {
      handler();
    }
  }
}

export default {
  NodeSDK,
};

