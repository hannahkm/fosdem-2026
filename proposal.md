# How to Instrument Go Without Changing a Single Line of Code

Submitted by: [Kemal Akkoyun](mailto:kemal.akkoyun@datadoghq.com)[Hannah S Kim (APM)](mailto:hannahs.kim@datadoghq.com)
Submitted to: [https://fosdem.org/2026/schedule/track/go/](https://fosdem.org/2026/schedule/track/go/)

---

Zero-touch observability for Go is finally becoming real. In this talk, we’ll walk through the different strategies you can use to instrument Go applications without changing a single line of code, and what they cost you in terms of overhead, stability, and security.

We’ll compare several concrete approaches and projects:

- eBPF-based auto-instrumentation, using OpenTelemetry’s Go auto-instrumentation agent:
    - [https://github.com/open-telemetry/opentelemetry-go-instrumentation](https://github.com/open-telemetry/opentelemetry-go-instrumentation)
    - [https://opentelemetry.io/docs/zero-code/go/autosdk/](https://opentelemetry.io/docs/zero-code/go/autosdk/)
    - [https://opentelemetry.io/docs/zero-code/obi/](https://opentelemetry.io/docs/zero-code/obi/)
- Compile-time manipulation, using tools that rewrite or augment Go binaries at build time, such as:
    - [https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation)
    - [https://github.com/Datadog/orchestrion](https://github.com/Datadog/orchestrion)

Beyond what exists today, we’ll look at how ongoing work in the Go runtime and diagnostics ecosystem could unlock cleaner, safer hooks for future auto-instrumentation, including:

- Further Runtime techniques, like shared-library injection, and binary trampolines, as used in OpenTelemetry, such as:
    - LD_PRELOAD magic
    - [https://github.com/open-telemetry/opentelemetry-injector](https://github.com/open-telemetry/opentelemetry-injector)
    - [https://github.com/open-telemetry/opentelemetry-specification/pull/4793\#issuecomment-3663805441](https://github.com/open-telemetry/opentelemetry-specification/pull/4793#issuecomment-3663805441)
- USDT (User Statically-Defined Tracing) probes, exploring how to add or generate USDT probe points for Go services (at build time or via injection) so that external tooling (eBPF, DTrace-style tools, etc.) can consume high-level events without source changes.
- Adding further “tracepoints” to recently added tracing facilities to the runtime:
    - runtime/trace and diagnostics primitives:
        - [https://pkg.go.dev/runtime/trace](https://pkg.go.dev/runtime/trace)
        - [https://go.dev/doc/diagnostics](https://go.dev/doc/diagnostics)
    - Proposals such as Go “flight recording” (Issue \#63185):
        - [https://github.com/golang/go/issues/63185](https://github.com/golang/go/issues/63185)

Throughout the talk, we’ll use benchmark results and small, realistic services to compare these strategies along three axes:

- Performance overhead (latency, allocations, CPU impact)
- Robustness and upgradeability across Go versions and container images
- Operational friction: rollout complexity, debugging, and failure modes

Attendees will leave with a clear mental model of when to choose eBPF, compile-time rewriting, runtime injection, or USDT-based approaches, how OpenTelemetry’s Go auto-instrumentation fits into that picture, and where upcoming runtime features might take us next. The focus is strongly practical and open-source: everything shown will be reproducible using publicly available tooling in the Go and OpenTelemetry ecosystems.
