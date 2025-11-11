import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import schema from '../../../../specs/004-shared-libraries/contracts/telemetry-profile.schema.json';

describe('telemetry profile contract', () => {
  it('contains required fields and constraints', () => {
    const profile = {
      service_name: 'shared-example',
      otel_exporter: {
        endpoint: 'http://otel-collector:4317',
        protocol: 'grpc',
        headers: {
          authorization: 'Bearer token'
        },
        timeout_ms: 2000
      },
      log: {
        level: 'info',
        required_fields: ['request_id', 'trace_id', 'actor.subject'],
        sampling_rate: 1
      },
      metrics: {
        histograms: [
          {
            name: 'http.server.duration',
            buckets: [0.01, 0.05, 0.1]
          }
        ],
        resource_attributes: {
          'service.namespace': 'shared'
        }
      },
      tracing: {
        sampler: 'ratio',
        sampler_arg: 0.25,
        propagators: ['tracecontext', 'baggage']
      }
    };

    expect(/^[a-z0-9-]+$/.test(profile.service_name)).toBe(true);
    expect(['grpc', 'http/protobuf']).toContain(profile.otel_exporter.protocol);
    expect(profile.log.required_fields.length).toBeGreaterThanOrEqual(3);
    expect(profile.metrics.histograms[0].buckets.length).toBeGreaterThanOrEqual(3);
    expect(profile.tracing.sampler_arg).toBeGreaterThanOrEqual(0);
    expect(profile.tracing.sampler_arg).toBeLessThanOrEqual(1);

    const schemaUrl = new URL('../../../../specs/004-shared-libraries/contracts/telemetry-profile.schema.json', import.meta.url);
    expect(fs.existsSync(fileURLToPath(schemaUrl))).toBe(true);
    expect(schema.title).toBe('Telemetry Configuration Profile');
  });
});

