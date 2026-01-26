# fosdem-2026

A continuation (AKA improvement) on our [Gophercon '25 talk](https://github.com/hannahkm/gopherconus-2025).

Load tests applications that are instrumented with:

1. Nothing (baseline)
2. Manual instrumentation ([OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/getting-started/))
3. OBI ([OpenTelemetry eBPF Instrumentation](https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation))
4. eBPF ([OpenTelemetry eBPF Autoinstrumentation](https://github.com/open-telemetry/opentelemetry-go-instrumentation))
5. Orchestrion ([Datadog toolchain instrumentation](https://github.com/datadog/orchestrion) with OpenTelemetry SDK under the hood)

Each scenario can be run multiple times and are, by default, sent to DataDog.

TODO: endpoint should be configurable so users can use Grafana as well.

## How to Use

Make sure Docker is running.

`go run . run --scenario [scenario]`, where `[scenario]` is one of default, manual, obi, ebpf, orchestrion, or all. If running `all`, all five scenarios will run in sequence.