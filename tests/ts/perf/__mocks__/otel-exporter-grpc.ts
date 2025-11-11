export class OTLPTraceExporter {
  constructor(options: unknown) {
    const handler = (globalThis as any).__otelGrpcMock;
    if (typeof handler === 'function') {
      handler(options);
    }
  }
}

export default {
  OTLPTraceExporter,
};

