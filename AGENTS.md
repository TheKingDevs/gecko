# AGENTS.md

## Idioma

- Comunique-se com o usuário em **português**, inclusive nas ferramentas (`question`, `todowrite`, etc.).
- Código e comentários: use **inglês padrão** (comercial), seguindo as convenções do projeto.

## Visão geral

Go toolchain source (golang/go mirror) on the go1.28 dev cycle, with an in-tree `simd` experiment layered on `master`. Almost all work happens under `src/`, which is the `std` module — run go commands from `src/`, not the repo root.

## Building

- Requires a bootstrap Go toolchain. Per `src/cmd/dist/README`: any release >= Go 1.26.0 and < Go 1.28 works. The system go 1.26.1 is sufficient; `./make.bash` auto-detects `go` on PATH.
- `src/go.mod` declares `go 1.28`. The system go (1.26.1) will hit `GOTOOLCHAIN=auto` and try to download go1.28.0, which fails offline. Run builds/tests with the in-tree toolchain once built, and set `GOTOOLCHAIN=local` when invoking the bootstrap go.
- Build with the experiment enabled: `GOEXPERIMENT=simd ./make.bash` (from `src/`). This bakes `simd` on for every later use of the produced `bin/go` — verify with `bin/go env GOEXPERIMENT` (`simd`) or `bin/go version` (`X:simd`). To test upstream behavior without simd you must rebuild the toolchain without that env var.
- `./make.bash` produces `bin/go` and `bin/gofmt` at the repo root; `./all.bash` additionally runs the full test suite.

## Testing

All from `src/` with the built `bin/go`:

- Single package: `bin/go test ./simd/...` (any std package likewise by its import path).
- Full suite (`go test std cmd` + extras): `bin/go tool dist test`; filter with `-run <re>` and list with `-list`. When the toolchain was built without simd, dist auto-registers `GOEXPERIMENT=simd go test simd` and `simd/archsimd/...` runs.
- Compiler/regression tests live in `test/` (executed by the `cmd/internal/testdir` package, part of `go test std cmd`). Run one file by relative path, e.g. `bin/go test cmd/internal/testdir -run 'Test/simd'` for `test/simd.go`. Each file's behavior comes from its header directive (`// run`, `// errorcheck`, etc.). New regression tests go in `test/`.

## The simd experiment (fork-specific)

- Everything simd is gated behind `goexperiment.simd` via `//go:build` tags — it does not compile without a simd-enabled toolchain.
- Architecture: `src/simd` (portable, vector-agnostic types) → `src/simd/archsimd` (per-arch impl: SVE on arm64, AVX on amd64, wasm) → `src/simd/internal/{bridge,spec,simdref}`. The `simd` package is experimental, not covered by the Go 1 compatibility promise.
- `archsimd` is full of generated code (`*_gen_*.go`, `types_*.go`, `ops_*.go`, `sve_arm64.go`, etc.). The generator lives in the nested module `src/simd/archsimd/_gen` (its own go.mod, requires go >= 1.26.5, plus golang.org/x/arch and x/tools) and runs via `go generate` in `archsimd` (`go run -C _gen . -w`).
- Regenerating **requires externally downloaded Intel XED and ARM AArch64 ISA XML data** (`_gen/fetch-xed.sh`, `_gen/fetch-arm64.sh`; the tool refuses to run without them). If that data isn't available, edit the generated files by hand following the existing generated-file patterns and formatting (gofmt output).
- arm64 is exercised via SVE only (code paths are `goexperiment.simd && arm64` gated). Upstream tracks this work too (go.dev/issue/79899, `gotip-linux-amd64-simd` builder).

## Other notes

- `misc/` is a separate go module (`go 1.22`); `test/` files and `doc/next` follow upstream Go relnote/API conventions.
- `.gitattributes` marks everything binary (`* -text`); `test/winbatch.go` enforces CRLF for `.bat` files.
- Fork changes are tracked in `CHANGELOG.md`, not in this file.