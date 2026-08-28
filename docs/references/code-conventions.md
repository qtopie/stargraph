# Go Language Coding Standards (Effective Go)

This document establishes the Go programming standards and conventions for this repository, adhering strictly to the official [Effective Go](https://go.dev/doc/effective_go) guide and standard Go ecosystem best practices.

---

## 1. Automated Tooling & Formatting Commands

All formatting, import sorting, and linting must be handled using standard tools. Whenever Go code is created or modified, execute the following commands:

- **Format Code (`gofmt`)**:
  ```bash
  gofmt -s -w <file_or_path>
  # Format entire project:
  gofmt -s -w .
  ```
- **Sort & Optimize Imports (`goimports`)**:
  ```bash
  goimports -w <file_or_path>
  # Optimize all project imports:
  goimports -w .
  ```
- **Static Analysis & Linting**:
  ```bash
  # Go standard static analyzer
  go vet ./...

  # Comprehensive linter suite (when configured)
  golangci-lint run
  ```
- **Dependency Hygiene & Testing**:
  ```bash
  # Prune unused dependencies and sync go.mod
  go mod tidy

  # Run tests with race detection
  go test -race ./...
  ```
- **Indentation**: Use tabs (`\t`) for indentation, not spaces.
- **Line Length**: Go has no rigid line length limit, but avoid excessively long lines. Wrap long expressions cleanly with proper indentation.

---

## 2. Commentary & Documentation

- **Doc Comments**:
  - Every exported (capitalized) package, type, constant, variable, and function/method **must** have a doc comment.
  - Doc comments must begin with the name of the declared item:
    ```go
    // Customer represents an account entity in the system.
    type Customer struct { ... }

    // CalculateTax computes total tax based on regional rates.
    func CalculateTax(amount float64) float64 { ... }
    ```
  - Use complete, well-formed sentences ending with a period.
- **Package Comments**:
  - Every package should have a package comment preceding the `package` clause in at least one file (typically `doc.go` or the primary file):
    ```go
    // Package storage provides persistent key-value store primitives.
    package storage
    ```

---

## 3. Naming Conventions

- **Package Names**:
  - Lowercase, single-word names (e.g., `transport`, `parser`, `bytes`).
  - No underscores, hyphens, or `mixedCaps`.
  - The package name should match its base directory name.
- **Getters & Setters**:
  - Do not prefix getters with `Get`.
    - Correct: `obj.Owner()` and `obj.SetOwner(...)`
    - Incorrect: `obj.GetOwner()`
- **Interface Names**:
  - Single-method interfaces should be named with the `-er` or `-or` suffix (e.g., `Reader`, `Writer`, `Formatter`, `Notifier`, `Closer`).
- **MixedCaps & Acronyms**:
  - Use `MixedCaps` / `camelCase` for identifiers (no snake_case).
  - Initialisms and acronyms must remain consistent in case:
    - Use `userID`, `httpURL`, `XMLHTTPRequest` or `xmlHTTPRequest` (not `userId`, `HttpUrl`, `XmlHttpRequest`).

---

## 4. Package Organization & Architecture

- **Single Responsibility & Cohesion**:
  - Each package must serve a single, clear domain or functional purpose.
  - **Prohibit Generic Horizontal Layer Packages (`domain`, `models`, `types`, `entities`, `interfaces`, `util`, `common`, `helpers`)**:
    - **Why `package domain` is an anti-pattern in Go**:
      1. *Semantic Emptiness*: In Go, the package identifier is part of every symbol call site (e.g. `domain.Node`, `domain.Document`). `domain` provides zero domain-specific context.
      2. *Grab-Bag & God Package*: Grouping all domain entities into a single package violates cohesion and SRP, quickly accumulating unrelated business concepts.
      3. *Circular Dependency Hell*: When logic packages across subdomains need to interact or reference specific types, a centralized `domain` or `models` package triggers package import cycles.
      4. *Java/DDD Layered Distortion*: In Java/C#, namespaces don't prefix call sites and circular dependencies between packages are often allowed; in Go, packages are modular units of compilation and design.
    - **Idiomatic Go Solution (Package by Feature / Subdomain)**:
      - Split by specific bounded capability: e.g., `graph.Node`, `graph.Edge`, `document.Chunk`, `search.Request`, `storage.KVStorage`.
      - Export types and interfaces where they naturally belong or at consumer boundaries.
- **Eliminate Stutter (Repetition)**:
  - Do not repeat the package name in exported types, functions, or constants:
    - Correct: `graph.Node`, `document.Chunk`, `search.Mode`, `http.Client`, `bytes.Buffer`
    - Incorrect: `graph.GraphNode`, `document.DocChunk`, `domain.DomainNode`, `http.HttpClient`
- **Standard Project Layout**:
  - `cmd/<app-name>/`: Application entrypoints (`main.go`). Keep `main()` minimal (parse flags/env, wire dependencies, start service).
  - `internal/`: Private implementation packages that must not be imported by other external modules (enforced by Go compiler).
  - `pkg/` or root packages: Exportable, reusable library code.
- **Minimal API Surface & Encapsulation**:
  - Keep internal helper functions, fields, and types unexported (lowercase).
  - Only export what consumers explicitly need to construct or interact with.
- **Acyclic Dependencies (No Package Cycles)**:
  - The package dependency graph must strictly be a Directed Acyclic Graph (DAG).
  - If package A and package B need to communicate, pass interfaces or extract shared definitions into an independent package.
- **`init()` Function Discipline**:
  - Keep `init()` minimal and deterministic. Avoid I/O, network calls, database connections, or hidden global side effects in `init()`.
  - Favor explicit initialization via constructors (`New...`) and dependency injection.
- **File Sizing & Granularity (Non-Mandatory Recommendations)**:
  - **Pragmatic Sizing**: Aim for **300 ~ 800 lines** per file as a comfortable balance for human readability and Agent context efficiency.
  - **High Cohesion Over Line Counts**: Keep related methods of the same core struct together even if the file reaches 800 ~ 1,200 lines. Avoid artificially fragmenting code across tiny files.
  - **Refactoring Cues**: When a file exceeds ~1,200 lines, consider extracting auxiliary concerns (e.g., `options.go`, `types.go`, `errors.go`) only if it improves clarity.

---

## 5. Control Flow & Structure

- **Guard Clauses & Early Return**:
  - Eliminate unnecessary `else` branches after `return`, `break`, `continue`, or `panic`.
  ```go
  // Preferred
  if err != nil {
      return err
  }
  return process()

  // Avoid
  if err != nil {
      return err
  } else {
      return process()
  }
  ```
- **Initialization in `if`**:
  - Scope helper variables to the `if` block where applicable:
  ```go
  if val, err := compute(); err != nil {
      return err
  } else if val > threshold {
      return process(val)
  }
  ```
- **For Loops**:
  - Use `_` for unused loop indices or values.
  - Be mindful of loop variable capture when spawning goroutines or taking pointers (Go 1.22+ handles per-iteration variables, but keep intentional scope).
- **Switch Statements**:
  - Prefer clean `switch` instead of chained `if-else if-else`.
  - Use type switches for interface inspection:
  ```go
  switch v := value.(type) {
  case string:
      fmt.Println("String:", v)
  case int:
      fmt.Println("Integer:", v)
  default:
      fmt.Printf("Unexpected type %T\n", v)
  }
  ```

---

## 6. Functions & Methods

- **Receiver Type Selection**:
  - Use **pointer receiver (`*T`)** if:
    - The method needs to mutate the receiver.
    - The receiver is a large struct.
    - Consistency: If some methods of `T` have pointer receivers, usually all should.
  - Use **value receiver (`T`)** if:
    - The type is small, immutable, or a basic primitive/map/func.
- **Multiple Return Values**:
  - Functions returning errors must place `error` as the last return value:
    ```go
    func FetchUser(id string) (*User, error)
    ```
- **Named Result Parameters**:
  - Use named results when they clarify documentation or when modifying returned values in `defer`.
  - Avoid naked `return` in non-trivial functions as it harms readability.

---

## 7. Data Types & Initialization

- **`new()` vs `make()`**:
  - `new(T)` allocates zeroed memory and returns `*T`.
  - `make(T, args...)` creates and initializes slices, maps, and channels (returns initialized `T`, not a pointer).
- **Zero-Value Usefulness**:
  - Design structs so their zero value is ready to use without an explicit constructor where feasible (e.g., `bytes.Buffer`, `sync.Mutex`).
- **Constructors**:
  - When initialization is needed, provide a factory function named `New` (if package is single-purpose, e.g. `ring.New()`) or `New<Type>` (e.g., `parser.NewConfig()`).
- **Composite Literals**:
  - Prefer keyed composite literals for structs with multiple fields:
  ```go
  cfg := Config{
      Host: "localhost",
      Port: 8080,
  }
  ```
- **Map Access**:
  - Always use the comma-ok idiom when presence needs verification:
  ```go
  if val, ok := cache[key]; ok {
      return val
  }
  ```

---

## 8. Interfaces

- **Accept Interfaces, Return Structs**:
  - Functions should generally accept the narrowest interface required and return concrete types.
- **Interface Segregation**:
  - Keep interfaces small, focused, and composed of smaller interfaces (`io.ReadWriter` composing `io.Reader` and `io.Writer`).

---

## 9. Concurrency

- **Share Memory By Communicating**:
  - Prefer channels and message passing to synchronize state over shared memory when appropriate.
- **Channel Discipline**:
  - Only the producer/sender should close a channel, never the consumer/receiver.
  - Never close a closed channel or send on a closed channel.
- **Context Propagation**:
  - Pass `ctx context.Context` as the first parameter for functions that perform I/O, network calls, or long-running background tasks.
  - Respect `ctx.Done()` for graceful cancellation.
- **Resource Cleanup**:
  - Always clean up spawned goroutines to prevent goroutine leaks.

---

## 10. Error Handling

- **Explicit Errors**:
  - Always check and handle errors. Do not silently discard with `_` unless explicitly safe and commented.
- **Error Context & Wrapping**:
  - Use `fmt.Errorf("operation failed: %w", err)` to wrap lower-level errors.
  - Inspect errors using `errors.Is` and `errors.As`.
- **Panic & Recover**:
  - Do not use `panic` for normal error control flow.
  - Reserve `panic` for truly unrecoverable programming errors / invariant failures during startup.
  - If a package uses `panic` internally, recover within the package boundary and return a proper `error`.

---

## 11. Resource Management (`defer`)

- Use `defer` immediately after resource allocation (e.g., mutex locking, file opening, network connections):
  ```go
  mu.Lock()
  defer mu.Unlock()

  f, err := os.Open(filename)
  if err != nil {
      return err
  }
  defer f.Close()
  ```
