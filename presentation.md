---
marp: true
theme: rose-pine
# theme: rose-pine-dawn
# theme: rose-pine-moon
math: mathjax
html: true

# columns usage: https://github.com/orgs/marp-team/discussions/192#discussioncomment-1516155
style: |
    .columns {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 1rem;
    }
    .comment {
        color: #888;
    }
    .medium {
        font-size: 3.5em;
    }
    .big {
        font-size: 5em;
    }
    table {
        font-size: 0.7em;
    }
    .centered-table {
        display: flex;
        justify-content: center;
        color: #aaa;
    }
    thead th {
        background-color: #e0e0e0;
        color:rgb(16, 16, 16);
    }
    tbody tr {
        background-color: transparent !important;
    }
    .hl {
        background-color:rgb(221, 185, 41);
        padding: 0.1em 0;
    }
    .replace {
        display: inline-flex;
        flex-direction: column;
        align-items: center;
        line-height: 1.2;
    }
    .replace .old {
        text-decoration: line-through;
        color: #aaa;
    }
    .replace .new {
        font-weight: bold;
    }
    .bottom-citation {
        position: absolute;
        bottom: 40px;
        left: 80px;
        right: 70px;
        text-align: center;
    }
    .vcenter {
        display: flex;
        justify-content: center;
        align-items: center;
        height: 100%;
    }
    section {
        align-content: start;
        padding-top: 50px;
    }
    section.vcenter {
        align-content: center;
    }
    section.hcenter {
        text-align: center;
    }
    section::after {
        top: 30px;
        bottom: auto;
        left: auto;
        right: 70px;
        font-size: 0.8em;
        color: #aaa;
    }
    header {
        top: 20px;
        bottom: auto;
        left: 30px;
        right: auto;
        font-size: 0.6em;
        color: #aaa;
    }
    footer {
        top: auto;
        bottom: 20px;
        left: 30px;
        right: auto;
        font-size: 0.6em;
        color: #aaa;
    }
    .center {
        text-align: center;
        margin-top: 175px;
    }
    a {
        color: #0066cc;
        text-decoration: underline;
    }
    section p,
    section li {
        font-size: 0.8em;
    }
    .centered-mermaid {
        display: flex;
        justify-content: center;
    }
---

<!-- _class: vcenter invert -->

# How to Instrument Go Without Changing a Single Line of Code

Hannah Kim, Kemal Akkoyun

FOSDEM 2026

---

<!-- paginate: true -->

<!-- _class: vcenter invert -->

# WHAT IS AUTO-INSTRUMENTATION

---

<!-- _class: vcenter -->

# About Us

**Hannah Kim**

- Software Engineer at Datadog
- Working on Go observability
- GopherCon US 2025 speaker

**Kemal Akkoyun**

- Staff Engineer at Datadog
- Observability and performance tooling
- Go enthusiast

---

<!-- _class: vcenter invert -->

# What is instrumentation?

---

<!-- _class: vcenter -->

<div class="vcenter">

<div style="text-align: center;">

## your application

</div>

</div>

---

<!-- _class: vcenter -->

<div class="vcenter">

<div style="text-align: center;">

## your application → your backend

</div>

</div>

---

<!-- _class: vcenter -->

<div class="vcenter">

<div style="text-align: center;">

## your application → your backend

### ???

### ???

</div>

</div>

---

<!-- _class: vcenter -->

<div class="vcenter">

<div style="text-align: center;">

## your application → your backend

### ???

### ???

### **LOGS**

(what happened)

</div>

</div>

---

<!-- _class: vcenter -->

<div class="vcenter">

<div style="text-align: center;">

## your application → your backend

### ???

### ???

### **LOGS**

(what happened)

### **METRICS**

(how much/fast things happened)

</div>

</div>

---

<!-- _class: vcenter -->

<div class="vcenter">

<div style="text-align: center;">

## your application → your backend

### ???

### ???

### **LOGS**

(what happened)

### **METRICS**

(how much/fast things happened)

### **TRACES**

(how things happened)

</div>

</div>

---

<!-- _class: vcenter invert -->

# What is auto-instrumentation?

---

<!-- _class: vcenter -->

# What is auto-instrumentation?

1. I want to know more about my code

---

<!-- _class: vcenter -->

# What is auto-instrumentation?

1. I want to know more about my code
2. I need to instrument it, but I'm too lazy to do it myself

---

<!-- _class: vcenter -->

# What is auto-instrumentation?

1. I want to know more about my code
2. I need to instrument it, but I'm too lazy to do it myself
3. <span class="medium">INSTRUMENTATION</span>

---

<!-- _class: vcenter -->

# What is auto-instrumentation?

1. I want to know more about my code
2. I need to instrument it, but I'm too lazy to do it myself
3. ???
4. <span class="big">Profit 💸💸💸</span>

---

<!-- _class: vcenter invert -->

# What is auto-instrumentation?

---

<!-- _class: vcenter -->

# What is auto-instrumentation?

**auto-instrumentation**: instrumenting your code (getting traces + data) without manual code changes

---

<!-- _class: vcenter -->

# What is auto-instrumentation?

<div class="columns">

<div>

### RUN TIME

- Happens at runtime
- Sometimes causes source code changes
- Meh with compiler languages like Go

</div>

</div>

**auto-instrumentation**: instrumenting your code (getting traces + data) without manual code changes

---

<!-- _class: vcenter -->

# What is auto-instrumentation?

<div class="columns">

<div>

### RUN TIME

- Happens at runtime
- Sometimes causes source code changes
- Meh with compiler languages like Go

</div>

<div>

### COMPILE TIME

- Happens at... compile time
- (Before run time)
- Works great with compiler languages like Go

</div>

</div>

**auto-instrumentation**: instrumenting your code (getting traces + data) without manual code changes

---

<!-- _class: vcenter -->

# Runtime Approaches

- iovisor/gobpf
- cilium/eBPF
- OpenTelemetry Auto-Instrumentation
- OpenTelemetry eBPF Instrumentation (OBI)
- Hooking
    - Shared library injection
    - Binary trampolining

**eBPF**: extended Berkeley packet filter

---

<!-- _class: vcenter -->

# How eBPF Works

<div class="centered-mermaid">

```mermaid
graph LR
    kernel[kernel]
    process[our process<br/>work]

    process --> kernel

    style kernel fill:#f9f,stroke:#ccc,stroke-width:2px
    style process fill:#bbf,stroke:#ccc,stroke-width:2px
    linkStyle default stroke:#aaa,stroke-width:2px
```

</div>

**eBPF**: extended Berkeley packet filter

---

<!-- _class: vcenter -->

# How eBPF Works

<div class="centered-mermaid">

```mermaid
graph LR
    kernel[kernel<br/>eBPF]
    process[our process<br/>work]
    hook[hook]

    process --> hook --> kernel

    style kernel fill:#f9f,stroke:#333,stroke-width:2px
    style hook fill:#ffb,stroke:#ccc,stroke-width:2px
    style process fill:#bbf,stroke:#333,stroke-width:2px
    linkStyle default stroke:#aaa,stroke-width:2px
```

</div>

**eBPF**: extended Berkeley packet filter

---

<!-- _class: vcenter invert -->

# OpenTelemetry eBPF Instrumentation (OBI)

---

<!-- _class: vcenter -->

# What is OBI?

<div class="columns">

<div>

**OBI** (OpenTelemetry eBPF Instrumentation) is a runtime instrumentation approach that:

- Uses eBPF to hook into Go runtime
- Extracts telemetry without code modification
- Part of OpenTelemetry ecosystem
- Production-ready and vendor-neutral
- Requires administrative privileges (root access)

</div>
<div>
</div>
</div>

---

<!-- _class: vcenter -->

# What is OBI?

<div class="columns">

<div>

**OBI** (OpenTelemetry eBPF Instrumentation) is a runtime instrumentation approach that:

- Uses eBPF to hook into Go runtime
- Extracts telemetry without code modification
- Part of OpenTelemetry ecosystem
- Production-ready and vendor-neutral
- Requires administrative privileges (root access)

</div>
<div class="centered-mermaid">

```mermaid
graph TB
    app["Your Go Application<br/>(no changes needed)"]
    ebpf[eBPF hooks]
    sidecar["OBI Sidecar Container<br/>- eBPF programs<br/>- OpenTelemetry exporter"]
    collector[OTel Collector]

    app --> ebpf
    ebpf --> sidecar
    sidecar --> collector

    style app fill:#bbf,stroke:#333,stroke-width:2px
    style ebpf fill:#ffb,stroke:#333,stroke-width:2px
    style sidecar fill:#bfb,stroke:#333,stroke-width:2px
    style collector fill:#fbb,stroke:#333,stroke-width:2px
    linkStyle default stroke:#aaa,stroke-width:2px
```

</div>

</div>

---

<!-- _class: vcenter -->

# OBI Configuration

```yaml
# obi-config.yaml
open_port: 8080
service:
    name: fosdem-obi
log_level: debug

otel_traces_export:
    endpoint: http://otel-collector:4318

prometheus_export:
    port: 9090
    path: /metrics

meter_provider:
    features:
        - application
```

---

<!-- _class: vcenter -->

# Compile Time Approaches

<div>

- Datadog Orchestrion
- OpenTelemetry Compile Time Instrumentation SIG

</div>

---

<!-- _class: vcenter -->

# Compile Time Flow

<div class="centered-mermaid">

```mermaid
graph LR
    A[source code] --> B
    subgraph B[compile time]
        direction LR
    end
    B --> F[executable]

    style A fill:#bbf,stroke:#333,stroke-width:2px
    style B fill:#ffb,stroke:#333,stroke-width:2px
    style F fill:#bbf,stroke:#333,stroke-width:2px
    linkStyle default stroke:#aaa,stroke-width:2px
```

</div>

---

<!-- _class: vcenter -->

# Compile Time Flow

<div class="centered-mermaid">

```mermaid
graph LR
    A[source code] --> B
    subgraph B[compile time]
        direction LR
        C[AST/IR]
    end
    B --> F[executable]

    style A fill:#bbf,stroke:#333,stroke-width:2px
    style B fill:#ffb,stroke:#333,stroke-width:2px
    style C fill:#fbb,stroke:#333,stroke-width:2px
    style F fill:#bbf,stroke:#333,stroke-width:2px
    linkStyle default stroke:#aaa,stroke-width:2px
```

</div>

**AST**: abstract syntax tree
**IR**: intermediate representation

---

<!-- _class: vcenter -->

# Compile Time Flow

<div class="centered-mermaid">

```mermaid
graph LR
    A[source code] --> B
    subgraph B[compile time]
        direction LR
        C[AST/IR] --> D[machine code]
    end
    B --> F[executable]

    style A fill:#bbf,stroke:#333,stroke-width:2px
    style B fill:#ffb,stroke:#333,stroke-width:2px
    style C fill:#fbb,stroke:#333,stroke-width:2px
    style D fill:#fbb,stroke:#333,stroke-width:2px
    style F fill:#bbf,stroke:#333,stroke-width:2px
    linkStyle default stroke:#aaa,stroke-width:2px
```

</div>

**AST**: abstract syntax tree
**IR**: intermediate representation

---

<!-- _class: vcenter -->

# Compile Time Flow

<div class="centered-mermaid">

```mermaid
graph LR
    A[source code] --> B
    subgraph B[compile time]
        direction LR
        C[AST/IR] --> D[machine code] --> E[linking]
    end
    B --> F[executable]

    style A fill:#bbf,stroke:#333,stroke-width:2px
    style B fill:#ffb,stroke:#333,stroke-width:2px
    style C fill:#fbb,stroke:#333,stroke-width:2px
    style D fill:#fbb,stroke:#333,stroke-width:2px
    style E fill:#fbb,stroke:#333,stroke-width:2px
    style F fill:#bbf,stroke:#333,stroke-width:2px
    linkStyle default stroke:#aaa,stroke-width:2px
```

</div>

**AST**: abstract syntax tree
**IR**: intermediate representation

---

<!-- _class: vcenter -->

# Compile Time Flow

<div class="centered-mermaid">

```mermaid
graph LR
    A[source code] --> B
    subgraph B[compile time]
        direction LR
        C[AST/IR] --> D[machine code] --> E[linking]
    end
    B --> F[executable]

    style A fill:#bbf,stroke:#333,stroke-width:2px
    style B fill:#ffb,stroke:#333,stroke-width:2px
    style C fill:#fbb,stroke:#333,stroke-width:2px
    style D fill:#fbb,stroke:#333,stroke-width:2px
    style E fill:#fbb,stroke:#333,stroke-width:2px
    style F fill:#bbf,stroke:#333,stroke-width:2px
    linkStyle default stroke:#aaa,stroke-width:2px
```

</div>

```bash
go run -toolexec 'orchestrion toolexec' .
```

**AST**: abstract syntax tree
**IR**: intermediate representation

---

<!-- _class: vcenter -->

# What is Orchestrion?

**Orchestrion** is a compile-time instrumentation approach that:

- Traces the AST created during compile time
- Injects Datadog instrumentation at specific nodes
- Updates executable file without source code changes
- Can be configured to add/remove instrumentation
- Compatible with OpenTelemetry

---

<!-- _class: vcenter -->

# Orchestrion Configuration

```yaml
# orchestrion.yaml
aspects:
    - id: make spans
      join-point:
          all-of:
              - package-name: main
              - function-body:
                    function:
                        - name: main
      advice:
          - prepend-statements:
                imports:
                    otel: go.opentelemetry.io/otel
                    context: context
                template: |-
                    tracer := otel.Tracer()
                    _, span := tracer.Start(context.Background, "orchestrion.handler")
```

---

<!-- _class: vcenter invert -->

# How do they compare?

---

<!-- _class: vcenter -->

<div class="centered-table">

| Approach           | CPU | Memory | # Errors |
| ------------------ | --- | ------ | -------- |
| Manual             |     |        |          |
| Auto (eBPF)        |     |        |          |
| Auto (OBI)         |     |        |          |
| Auto (Orchestrion) |     |        |          |

</div>

```bash
TODO(hannah): add numbers +/- to table above, add more columns as necessary
```

---

<!-- _class: vcenter invert -->

# Who wins?

---

<!-- _class: vcenter -->

# Comparison Matrix

<div class="centered-table">

| Approach           | Performance | Stability | Security | Portability |
| ------------------ | ----------- | --------- | -------- | ----------- |
| Auto (eBPF)        |             |           |          |             |
| Auto (OBI)         |             |           |          |             |
| Auto (Orchestrion) |             |           |          |             |

</div>

---

<!-- _class: vcenter -->

# Comparison Matrix

<div class="centered-table">

| Approach           | Performance | Stability | Security | Portability |
| ------------------ | ----------- | --------- | -------- | ----------- |
| Auto (eBPF)        | ⚠           |           |          |             |
| Auto (OBI)         | ⚠           |           |          |             |
| Auto (Orchestrion) | ⚠           |           |          |             |

</div>

---

<!-- _class: vcenter -->

# Comparison Matrix

<div class="centered-table">

| Approach           | Performance | Stability | Security | Portability |
| ------------------ | ----------- | --------- | -------- | ----------- |
| Auto (eBPF)        | ⚠           | ⚠         |          |             |
| Auto (OBI)         | ⚠           | ⚠         |          |             |
| Auto (Orchestrion) | ⚠           | ✅        |          |             |

</div>

---

<!-- _class: vcenter -->

# Comparison Matrix

<div class="centered-table">

| Approach           | Performance | Stability | Security | Portability |
| ------------------ | ----------- | --------- | -------- | ----------- |
| Auto (eBPF)        | ⚠           | ⚠         | ⚠        |             |
| Auto (OBI)         | ⚠           | ⚠         | ⚠        |             |
| Auto (Orchestrion) | ⚠           | ✅        | ✅       |             |

</div>

---

<!-- _class: vcenter -->

# Comparison Matrix

<div class="centered-table">

| Approach           | Performance | Stability | Security | Portability |
| ------------------ | ----------- | --------- | -------- | ----------- |
| Auto (eBPF)        | ⚠           | ⚠         | ⚠        | ⚠           |
| Auto (OBI)         | ⚠           | ⚠         | ⚠        | ⚠           |
| Auto (Orchestrion) | ⚠           | ✅        | ✅       | ✅          |

</div>

---

<!-- _class: vcenter -->

# The Winner?

<div class="centered-table">

| Approach           | Performance | Stability | Security | Portability |
| ------------------ | ----------- | --------- | -------- | ----------- |
| Auto (eBPF)        | ⚠           | ⚠         | ⚠        | ⚠           |
| Auto (OBI)         | ⚠           | ⚠         | ⚠        | ⚠           |
| Auto (Orchestrion) | ⚠           | ✅        | ✅       | ✅          |

</div>

**It depends on your use case!**

eBPF/OBI: Great for <span class="hl">runtime flexibility</span>
Orchestrion: Great for <span class="hl">stability and security</span>

---

<!-- _class: vcenter invert -->

# The future

---

<!-- _class: vcenter -->

# The future

We asked, the Go team answered...

- **golang/go#63185** – Flight recording (released in Go 1.25)

---

<!-- _class: vcenter -->

# The future

We asked, the Go team answered...

- **golang/go#63185** – Flight recording (released in Go 1.25)

Go Compile Time Instrumentation SIG

- Tuesdays 12:30-1:30PM EST

---

<!-- _class: vcenter -->

# The future

We asked, the Go team answered...

- **golang/go#63185** – Flight recording (released in Go 1.25)

Go Compile Time Instrumentation SIG

- Tuesdays 12:30-1:30PM EST

---

<!-- _class: vcenter invert -->

# Final thoughts

---

<!-- _class: vcenter -->

# Final thoughts

1. Instrumentation is helpful and important

---

<!-- _class: vcenter -->

# Final thoughts

1. Instrumentation is helpful and important
2. Auto-instrumentation is EASY

---

<!-- _class: vcenter -->

# Final thoughts

1. Instrumentation is helpful and important
2. Auto-instrumentation is EASY
3. What are YOU going to do next?

---

<!-- _class: vcenter invert -->
<!-- footer: "" -->

# Conclusion

---

<!-- _class: vcenter invert -->
<!-- footer: "" -->

<style scoped>
.columns {
    height: 100%;
    align-items: center;
}
.columns > div:first-child {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: flex-start;
    padding-left: 50px;
}
.contact-info {
    font-size: 0.7em;
    line-height: 1.6;
    margin-top: 20px;
}
</style>

<div class="columns">

<div>

# Thanks

<div class="contact-info">

**Hannah Kim**

- hannahkm.github.io
- linkedin.com/in/hannah-kim24/

**Kemal Akkoyun**

- @kakkoyun
- github.com/kakkoyun

</div>

</div>
<div>

![width:500](./assets/slides_qr_code.png)

</div>

</div>

---

<style scoped>
p { font-size: 0.5em; line-height: 1.4; }
</style>

# References

\[1\]
