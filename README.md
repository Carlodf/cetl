# CETL — Composable ETL Building Blocks in Go

CETL provides small, orthogonal components for building **streaming ETL pipelines** in Go.  
The focus is on **composability**, **observability**, and **zero-copy streaming** (no loading whole files in memory).

The project is structured around three layers:

```
+------------+      +-------------+      +----------------+
|  Openers   | ---> |  Connectors | ---> |  Transformers  |
+------------+      +-------------+      +----------------+
```

## Openers
Openers describe **where data comes from**.  
Example: reading local files that match a glob pattern.

```go
ops, err := openers.RegularFileOpenerFactory("data/*.psv")
```

## Connectors
Connectors combine one or more openers into a **single streaming source**.  
The `muxReader` allows multiple input sources to be read **sequentially as one stream**, while still exposing:

- **Current()** → which source is active, and byte offset
- **AwaitBoundary()** → when the stream switches to the next source

```go
mux := connectors.NewMuxReader(ctx, ops)
defer mux.Close()

b := make([]byte, 4096)
n, err := mux.Read(b)
```

## Transformers
Transformers parse and process the stream (e.g. CSV/PSV parsing, filtering, validation) and eventually feed an in-memory view or load into a database.  
This layer is intentionally decoupled so ETL logic develops **after** source handling is stable.

---

## Why CETL?

- **Streaming-first**: designed for files too large to load in memory.
- **Composable**: pieces fit together but do not depend on each other.
- **Transparent boundary tracking**: you always know *which* input produced *which* output.
- **Testable**: Openers, Connectors, and Transformers can be tested independently.

---

## Example: Read `.psv` files as a unified row stream

```go
ops, _ := openers.RegularFileOpenerFactory("logs/*.psv")

mux := connectors.NewMuxReader(context.Background(), ops)
defer mux.Close()

csvr := csv.NewReader(mux)
csvr.Comma = '|'

for {
    row, err := csvr.Read()
    if errors.Is(err, io.EOF) {
        break
    }
    fmt.Println(mux.Current().Name, row)
}
```

Example output:

```
logs/2024-10-01.psv [field1 field2 field3]
logs/2024-10-01.psv [field1 field2 field3]
logs/2024-10-02.psv [field1 field2 field3]
```

---

## Status

This library is **under active development**.  
API may evolve as Transformer and Loader layers are completed.

---

## License

Apache 2.0
