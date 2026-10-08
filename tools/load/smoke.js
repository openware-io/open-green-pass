import http from 'k6/http';
import { check, fail } from 'k6';

const base = (__ENV.BASE_URL || 'http://127.0.0.1:30080').replace(/\/$/, '');
const allowlist = /^https?:\/\/(127\.0\.0\.1|localhost)(:\d+)?$/;

if (!allowlist.test(base) && __ENV.ALLOW_REMOTE !== 'true') {
  fail('BASE_URL must be a localhost GreenPass endpoint; set ALLOW_REMOTE=true only for an approved isolated environment');
}

export const options = {
  vus: Number(__ENV.VUS || 1),
  duration: __ENV.DURATION || '30s',
  tags: { harness: 'gp-smoke-v1' },
};

export default function () {
  const params = { tags: { target: 'greenpass-control-plane' } };
  check(http.get(`${base}/api/healthz`, params), { 'health endpoint is ready': (r) => r.status === 200 });
  check(http.get(`${base}/api/metrics`, params), { 'metrics endpoint is available': (r) => r.status === 200 });
}
