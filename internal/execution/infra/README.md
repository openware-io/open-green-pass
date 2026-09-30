# execution infra

执行域 infra：Temporal 工作流（workflow/activity）、执行沙箱调度、真实依赖访问。禁 gorm/temporal 于 application，SDK 仅在此层。

## GP2-01 heterogeneous runner boundary

`runner` package is the scenario-to-resource adapter boundary. It currently
provides deterministic plans for API (`k8s-sandbox`), Web E2E/visual/performance
(`browser`), mobile E2E/compatibility/weak-network (`device`), and load testing
(`load`). The plan contains the image, command and resource class; a deployment
specific pool executor owns the actual browser/device/sandbox lifecycle.

`GP_RUNNER_TYPE=scenario` selects `ScenarioRunner` in server and worker. This
mode is intended for adapter integration and local development; production
deployments must inject a sandbox/pool `CommandExecutor` rather than execute
commands in the control-plane process. `GP_RUNNER_TYPE=k8s` remains the P1
Kubernetes Job runner path.

`pool` provides the GP2-08 resource-pool registry: pools advertise resource
capabilities and capacity, are selected by capability, and support reserve/
release. The current implementation is an in-process reference for tests;
persistence and heartbeats can be backed by Redis or PostgreSQL when the
cluster is available.
