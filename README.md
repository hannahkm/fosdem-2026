# fosdem-2026

[![Build Presentation](https://github.com/hannahkm/fosdem-2026/actions/workflows/slides.yml/badge.svg)](https://github.com/hannahkm/fosdem-2026/actions/workflows/slides.yml)

A continuation (AKA improvement) on our [Gophercon '25 talk](https://github.com/hannahkm/gopherconus-2025).

Load tests applications that are instrumented with:

1. Nothing (baseline)
2. Manual instrumentation ([OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/getting-started/))
3. OBI ([OpenTelemetry eBPF Instrumentation](https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation))
4. eBPF ([OpenTelemetry eBPF Autoinstrumentation](https://github.com/open-telemetry/opentelemetry-go-instrumentation))
5. Orchestrion ([Datadog toolchain instrumentation](https://github.com/datadog/orchestrion) with OpenTelemetry SDK under the hood)
6. libstabst (USDT probes via [salp](https://github.com/mmcshane/salp)/[libstapsdt](https://github.com/sthima/libstapsdt) + bpftrace)
7. usdt (Native USDT probes via [Go fork with USDT support](https://github.com/kakkoyun/go/tree/poc_usdt))

Each scenario can be run multiple times and are, by default, sent to DataDog.

### Scenario Requirements

| Scenario    | Go Version  | Environment | Notes                                    |
| ----------- | ----------- | ----------- | ---------------------------------------- |
| default     | 1.25.x      | Any         | No instrumentation                       |
| manual      | 1.25.x      | Any         | Manual OTel SDK                          |
| obi         | 1.25.x      | Linux       | Requires eBPF                            |
| ebpf        | 1.25.x      | Linux       | Requires eBPF                            |
| orchestrion | 1.25.x      | Any         | Compile-time injection                   |
| libstabst   | **1.23.x**  | **Linux**   | salp library incompatible with Go 1.25.x |
| usdt        | Custom fork | Linux       | Requires Linux kernel with USDT support  |

**Notes:**

- The `libstabst` scenario uses the salp library which has generics compatibility issues with Go 1.25+. The Dockerfile defaults to Go 1.23.6.
- USDT-based scenarios (`libstabst`, `usdt`) require a Linux environment with eBPF support. Docker Desktop on macOS has limited eBPF support.
- The `usdt` scenario uses a custom Go fork that is still a proof-of-concept.

See [app/libstabst/README.md](app/libstabst/README.md) and [app/usdt/README.md](app/usdt/README.md) for detailed known issues and workarounds.

## How to Use

Make sure Docker is running.

`go run . run --scenario [scenario]`, where `[scenario]` is one of default, manual, obi, ebpf, orchestrion, or all. If running `all`, all five scenarios will run in sequence.

## Quick Start

```bash
make install   # Install dependencies
make build     # Generate PDF
make watch     # Live preview with hot reload
```

## Available Targets

| Target        | Description                  |
| ------------- | ---------------------------- |
| `build`       | Generate PDF (default)       |
| `html`        | Generate HTML version        |
| `watch`       | Live preview with hot reload |
| `lint`        | Run markdown linting         |
| `format`      | Format markdown files        |
| `check-typos` | Check for typos              |
| `fix-typos`   | Fix typos automatically      |
| `clean`       | Remove generated files       |
| `install`     | Install npm dependencies     |

## Code Quality

This project uses automated tools to maintain quality:

| Tool                                                       | Purpose          | Config File          |
| ---------------------------------------------------------- | ---------------- | -------------------- |
| [markdownlint](https://github.com/DavidAnson/markdownlint) | Markdown linting | `.markdownlint.yaml` |
| [Prettier](https://prettier.io/)                           | Code formatting  | `.prettierrc`        |
| [typos](https://github.com/crate-ci/typos)                 | Spell checking   | `.typos.toml`        |

All checks run automatically in CI on pull requests.

## Download

Get the latest PDF from [GitHub Actions](https://github.com/hannahkm/fosdem-2026/actions/workflows/build.yml) - click on the latest successful run and download the `presentation-pdf` artifact.
