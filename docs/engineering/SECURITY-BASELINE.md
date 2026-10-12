# GreenPass security baseline

This document records the executable GP3-07 baseline. It does not claim that an external KMS, production network allow-list, or database role rollout has been completed.

## Trusted database boundary

- Production requires `GP_TRUSTED_DB_DSN`; it must not equal `GP_DB_DSN`.
- The trusted login is provisioned outside application migrations and inherits the `gp_trusted_writer` group role.
- `gp_server` and `gp_worker` runtime logins receive read access only to `aud_event`, `cost_line_item`, `gate_result`, and `gate_rule`.
- Migration `000013_trusted_permissions` forces RLS and blocks update/delete on append-only trusted tables.
- Applying role grants to an existing database requires a reviewed DBA change and a rollback rehearsal. The application must not create passwords or rotate production credentials.

## Secret references and logs

- Model credentials are opaque `secret://`, `k8s://`, or `vault://` references with a provider/name path. Plain values, credential-bearing URLs, and environment-variable references are rejected at the AI gateway.
- Provider adapters resolve a reference only at call time. Secret values must not be persisted in model, audit, cost, run, or case records.
- The process logger automatically replaces structured attributes whose keys identify authorization, token, secret, password, API key, prompt/response body, or phone data.
- Callers must still log identifiers and error classes instead of request bodies, provider responses, DSNs, or resolved secret values.

## Runner sandbox and network

- Runner Jobs use a dedicated service account, disable token automount, run as a numeric non-root user, use `RuntimeDefault` seccomp, drop all Linux capabilities, forbid privilege escalation, and use a read-only root filesystem.
- The controller service account may create and observe Jobs only through reviewed RBAC. It must not be reused by runner Jobs.
- The Helm NetworkPolicy currently restricts ingress but does not claim egress isolation. Before production, define DNS plus explicit provider/object-store/repository destinations, test those allow-lists in a staging namespace, and only then enable default-deny egress.
- Untrusted test commands must execute in runner Jobs or a dedicated pool, never in the server or worker container.

## Release checks

Run `go test ./...`, `go vet ./cmd/... ./internal/... ./pkg/...`, `helm lint deploy/helm/gp -f deploy/helm/gp/values-kind.yaml`, and the migration contract checks. Production acceptance additionally requires database role integration tests, image/SBOM scanning, and a staged egress-policy exercise.
