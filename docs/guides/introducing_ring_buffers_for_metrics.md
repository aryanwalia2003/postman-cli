# 📈 ReqX Architecture: High-Performance Metrics Pipeline

To handle millions of requests per second without the metrics collection becoming a bottleneck, ReqX uses a **Sharded, Lock-Free Ring Buffer** architecture. 

## Architecture Diagram

```mermaid
graph TD

%% Styling
classDef worker fill:#81c784,stroke:#2e7d32,stroke-width:2px,color:#fff;
classDef buffer fill:#64b5f6,stroke:#1565c0,stroke-width:2px,color:#fff;
classDef aggregator fill:#ffb74d,stroke:#ef6c00,stroke-width:2px,color:#fff;
classDef system fill:#f5f5f5,stroke:#9e9e9e,stroke-dasharray: 5 5;

%% Entry
A[🚀 Incoming Response Metrics]

%% Workers
subgraph Workers ["🔄 Self-Driven Workers (Producers)"]
    B1[Worker 1]:::worker
    B2[Worker 2]:::worker
    B3[Worker N]:::worker
end

%% Routing/Mapping Logic
C{{"⚖️ Shard Mapping<br/>(WorkerID % NumShards)"}}

%% Sharded Ring Buffers
subgraph Buffers ["📦 Lock-Free Ring Buffers (Shared Memory)"]
    RB1[Ring Buffer - Shard 1]:::buffer
    RB2[Ring Buffer - Shard 2]:::buffer
    RB3[Ring Buffer - Shard N]:::buffer
end

%% Local Aggregation
subgraph LocalAgg ["📊 Shard-Local Aggregators (Consumers)"]
    SA1["Aggregator 1<br/>(Local Map/Histogram)"]:::aggregator
    SA2["Aggregator 2<br/>(Local Map/Histogram)"]:::aggregator
    SA3["Aggregator N<br/>(Local Map/Histogram)"]:::aggregator
end

%% Finalization
FA["🏁 Final Global Aggregator<br/>(Thread-Safe Merge + Report Generation)"]:::system

%% Connections
A --> B1 & B2 & B3
B1 & B2 & B3 --> C
C -.->|"Atomic Write"| RB1
C -.->|"Atomic Write"| RB2
C -.->|"Atomic Write"| RB3

RB1 -->|"Single Consumer"| SA1
RB2 -->|"Single Consumer"| SA2
RB3 -->|"Single Consumer"| SA3

SA1 & SA2 & SA3 -->|"Batched Merge"| FA

%% Performance Note
Note[<b>Performance Note:</b> This sharded, lock-free architecture eliminates global lock contention,<br/>allowing ReqX to handle 1M+ RPS with sub-microsecond metric overhead.]
style Note fill:#fff9c4,stroke:#fbc02d,stroke-width:1px,color:#333;
FA --- Note
```

## Why this approach?

1. **Lock-Free Producers**: Workers don't wait for a global mutex to record a metric. They write to a assigned shard buffer using atomic pointer increments.
2. **Cache-Efficiency**: Each aggregator (consumer) runs on its own core/goroutine, reducing CPU cache-line bouncing (False Sharing).
3. **Batched Processing**: Instead of updating the global state for every request, metrics are aggregated locally in shards and merged in bulk at the end of the test runtime.
4. **Predictable Memory**: Ring buffers have a fixed size, preventing memory spikes during extreme load bursts.
