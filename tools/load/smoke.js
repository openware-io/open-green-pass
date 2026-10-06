import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: Number(__ENV.VUS || 1),
  duration: __ENV.DURATION || '30s',
};

export default function () {
  const base = __ENV.BASE_URL || 'http://127.0.0.1:30080';
  check(http.get(`${base}/api/healthz`), { 'health endpoint responded': (r) => r.status > 0 });
  check(http.get(`${base}/api/metrics`), { 'metrics endpoint responded': (r) => r.status > 0 });
}
