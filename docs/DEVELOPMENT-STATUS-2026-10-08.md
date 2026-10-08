# GreenPass development status audit

This status deliberately separates implemented code from integrated and
verified capabilities. A pure function, in-memory adapter, fake provider,
local script, or unit test is not a production or kind acceptance result.

## Recent code batches

| Commit | Actual code | Not completed by this commit |
|---|---|---|
| `fd0be0a` + `5d35eca` | PostgreSQL pool lease renewal with generation fencing; RunService renews during execution and cancels on stale generation | No kind PostgreSQL lease/recovery evidence; no real multi-cluster integration |
| `66fd0d7` | Redis dead-letter stream write + ACK path for malformed schedule payloads | No Redis integration/recovery evidence; no operator replay workflow |
| `5a06519` | Provider-independent placement dry-run validation | No API, persistent placement registry, migration, or real cutover |
| `3c82241` | Bounded read-only audit verifier service | No API/CLI wiring to this service; no external anchor |
| `6f0b05d` | Retry exhaustion state calculation | No durable production outbox worker or provider delivery |
| `470d75e` | Localhost-safe k6 smoke harness and runbook | No load execution, SLO, or capacity result |
| `6effaba` | GP-only backup/restore scripts and runbook | Scripts not executed; no backup store, encryption, RPO/RTO, or recovery evidence |

## Rules for subsequent reporting

1. “代码完成” means the implementation exists and its applicable local tests
   pass.
2. “集成完成” additionally requires the real PG/Redis/Temporal/kind path to
   execute through the production wiring.
3. “阶段完成” additionally requires the task DoD and evidence in the plan.
4. Fake, memory, unit-only, dry-run, or unexecuted scripts must be labelled as
   such and must never be reported as a completed production capability.
