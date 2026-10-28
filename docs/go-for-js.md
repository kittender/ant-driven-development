# Go for a JavaScript developer

Only what you need to read and change this codebase. Every point points at a file here.

## Daily commands

| You know | In Go |
|---|---|
| `npm install` | nothing: dependencies are fetched on build. `go mod tidy` cleans `go.mod` |
| `npm test` / `vitest` | `go test ./...` (`./...` = every package below here) |
| `vitest -t name` | `go test ./internal/core -run TestParseTarget -v` |
| `vitest --watch` | none built in; `go test ./...` is fast enough |
| `node script.js` | `go run ./cmd/add-eval` |
| `tsc` | `go build ./...` (also `go vet ./...` for likely bugs) |
| prettier | `gofmt -w .` (one style, no options, nobody argues) |
| a bundle | `go build -o bin/add-eval ./cmd/add-eval`: one static binary |

## The language, by analogy

**Packages are folders.** Every file in `internal/core/` starts with `package core` and they all see each other. Capitalised names are exported (`ParseTarget`), lower-case ones are private to the package (`checkRelPath`). There is no `export` keyword.

**Structs instead of objects, no classes.** `type Target struct { ... }` is a plain data shape. Methods are attached from outside: `func (t Target) String() string`. `t` is like `this`, named explicitly.

**Value vs pointer.** `func (t Target)` gets a *copy*; `func (s *Session)` gets a pointer and can mutate. Assigning a struct copies it (`g := p.Genome` in `eval/run.go` makes a new genome value). Slices and maps inside it are *not* deep-copied — see the comment in `core_test.go` `TestGenomeValidate`.

**Interfaces are implicit.** `model.Provider` is two methods. `Ollama`, `Anthropic` and `Fake` never say "implements": having the methods is enough. Like TypeScript's structural typing, but checked at compile time everywhere.

**Errors are values, not exceptions.** Functions return `(result, error)`. You check `if err != nil { return ..., err }` right away. It is verbose; it is also why every failure path in the harness is visible.
- `fmt.Errorf("dictionary: %w", err)` wraps an error, like `new Error("…", { cause })`.
- `errors.As(err, &pe)` is `err instanceof ParseError` through the wrapping (see `model/gateway.go`).

**`defer` is `finally`.** `defer resp.Body.Close()` runs when the function returns, whatever path it takes.

**No `undefined`.** Every type has a zero value: `""`, `0`, `false`, `nil` for pointers/slices/maps. `core.Action{}` is valid and empty. A missing JSON field decodes to the zero value.

**JSON via struct tags.** `` `json:"may_deposit"` `` maps a field to a key. `dec.DisallowUnknownFields()` is like a Zod `.strict()`: unknown keys are an error. That is how the harness "refuses settings it does not recognise".

**Strings are UTF-8 bytes.** `len("🧪")` is 4. A `rune` is one Unicode code point; `utf8.RuneCountInString` counts them. See `core/emoji.go`.

## Concurrency

| JavaScript | Go | Where |
|---|---|---|
| `async` function | any function run with `go f()` (a goroutine) | `eval/run.go` |
| a queue + `Promise.all` with a pool | a channel of jobs, N goroutines reading it, a `sync.WaitGroup` | `eval/run.go` |
| `AbortController` / `signal` | `context.Context`; `context.WithTimeout` | `runner/runner.go` |
| Ctrl-C handler | `signal.NotifyContext` | `cmd/add-eval/main.go` |
| nothing (single thread) | `sync.Mutex` around shared state | `model/fake.go`, `main.go` |

Run tests with `-race`: it catches two goroutines touching the same variable unsafely.

## Testing

- A test is `func TestXxx(t *testing.T)` in a `_test.go` file. `t.Errorf` records a failure and continues; `t.Fatalf` stops this test.
- **Table-driven tests** are the idiom (`test.each`): a slice of cases and one loop. See `core/core_test.go`.
- `t.Helper()` makes failures point at the caller.
- `httptest.NewServer` runs a real HTTP server in-process: `model/model_test.go` tests the Ollama and Anthropic adapters without the network.
- **Fuzzing is built in**: `go test ./internal/model -fuzz=FuzzParseText -fuzztime=30s` throws random inputs at the parser.

## Generics

Available and used sparingly. `loadWith[T any](path, load func(io.Reader) (T, error))` in `main.go` is the only one here: one helper that opens a file for any loader.

## Reading order for this repo

1. `internal/core/target.go`, `dictionary.go` — plain types and validation.
2. `internal/tools/gateway.go` — the enforcement rules, short and central.
3. `internal/runner/runner.go` — the session loop.
4. `internal/model/parse.go`, `ollama.go` — talking to models.
5. `internal/eval/run.go` — goroutines and channels in practice.
