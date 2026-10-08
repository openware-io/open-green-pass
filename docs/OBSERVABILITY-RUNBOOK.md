# GreenPass observability runbook

This runbook describes evidence collection only. It deliberately does not
define availability, latency, or error-budget SLOs; those values belong to the
deployment owner and must be supplied externally.

## Metrics

The server exposes Prometheus text format at `GET /metrics`. Metric names and
allowed labels are defined in `internal/platform/metrics/catalog.go`. Labels
must remain bounded: do not add team, user, run, case, request, or arbitrary
provider identifiers.

The recording rules in `deploy/observability/prometheus-rule.yaml` are optional
and require Prometheus Operator. The dashboard is a portable Grafana template;
replace the namespace variable with the target environment.

## Evidence collection

```powershell
kubectl -n gp port-forward svc/gp-server 18080:8080
curl.exe http://127.0.0.1:18080/metrics
```

Capture the metrics snapshot together with deployment revision, pod image
digests, and the load-harness parameters. Never include authorization headers,
tokens, prompts, or response bodies in the evidence bundle.

## Failure triage

1. Check `/readyz` and pod restart counts.
2. Check `gp_dependency_errors_total` grouped by the bounded `dependency` label.
3. Check queue wait and worker activity rates; compare with the same harness
   parameters and observation window.
4. Inspect Redis, PostgreSQL, and Temporal health independently. Do not restart
   or mutate the `open-im-local` namespace as part of GreenPass triage.

## Load harness

`tools/load/smoke.js` is a versioned k6 smoke harness. It calls only the
GreenPass health and metrics endpoints, defaults to `localhost`, and rejects a
remote target unless `ALLOW_REMOTE=true` is explicitly supplied for an approved
isolated environment. It never targets `open-im-local`.

```powershell
k6 run -e BASE_URL=http://127.0.0.1:30080 -e VUS=1 -e DURATION=30s tools/load/smoke.js
```

The harness intentionally has no SLO thresholds until D-11 is decided. Record
the command, harness version, Helm revision, image digests and `/metrics`
snapshot together; do not treat a smoke run as a capacity conclusion.
