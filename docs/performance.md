# Performance Benchmarks (Future Work)

TailLaunch does not currently publish benchmark numbers. This page records a
small, repeatable plan for future performance work without implying guarantees
about startup time, memory use, or network throughput.

## Planned measurements

- launcher startup and time to the connected state
- time from **Open App** to the first rendered page
- idle and active memory for Auto, Normal, and Low memory modes
- CPU use while idle and while proxying a representative private app
- proxy throughput and latency for HTTP and HTTPS CONNECT traffic
- cleanup time and whether temporary session directories are removed

## Planned matrix

Record OS, architecture, Go version, TailLaunch version, browser/version,
memory mode, persistence setting, target type, network conditions, and whether
the target is reached through tailnet routing or direct routing. Run repeated
measurements on clean sessions and report ranges rather than a single best
case.

Benchmark results should be added with the command, environment, raw output,
and date. Until then, choose memory mode based on the machine and workload;
the GUI's Auto mode remains the default.
