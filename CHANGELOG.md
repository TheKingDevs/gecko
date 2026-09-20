# Changelog

Fork changes layered on the upstream `golang/go` go1.28 dev cycle. This file
logs modifications that are specific to gecko and are not part of upstream Go.

## Unreleased

### Entry: module system kept internal-only (no user-facing mod)

**Título / Title:** the module machinery remains in the tree only as internal infrastructure (building/testing the std toolchain from `src/`, `GKTOOLCHAIN=auto` toolchain downloads, and gpm's interop with Go-ecosystem packages); it is no longer exposed to users. `gecko mod`, `gecko get` and the `help modules`/`help go.mod` topics are gone.

**Descrição / Description:**

- Registered help topics `modules` and `go.mod` (`modload.HelpModules`, `modload.HelpGoMod`) are removed from `cmd/go/main.go`, so `gecko help modules`/`gecko help go.mod`/`gecko help mod`/`gecko help get` now report "unknown help topic"; the now-orphaned `cmd/go/internal/modload/help.go` was deleted.
- User-facing command docs no longer point at the removed topics/commands: `help importpath`, `gecko build`, `gecko install`, `gecko run`, `gecko list`, `gecko work`, `gecko fmt` and `gecko fix` texts updated (project/gpm pointers instead of `'gecko help modules'`); error messages in `modload/init.go` drop the `see 'gecko help modules'`/`'gecko help mod init'` advice.
- `cmd/go/testdata/script/help.txt` no longer asserts help for `mod`/`get`; the `mod_help.txt` script (entirely about `go help mod`/`go help get`) is deleted. Scripts that still drive `gecko mod …` remain skipped via the `usesModCommand` harness filter.
- Modules stay in use internally: the std library is still the module in `src/`, `GKTOOLCHAIN=auto` downloads toolchains through the module proxy, and `cmd/gpm` reads third-party `go.mod` files as dependency manifests.

**Hash do commit / Commit hash:** `11f542c2`

**Mensagem do commit / Commit message:**
```
gecko: use GK* env vars for toolchain config, confine modules to internal
```

### Entry: gecko environment variables (GKROOT, GKPATH, GKHOME, …)

**Título / Title:** the toolchain no longer uses the standard Go `GO*` environment variables. Every variable the command manages or the compiler/linker reads is renamed to a `GK*` counterpart (`GKROOT`, `GKPATH`, `GKCACHE`, `GKENV`, `GKOS`, `GKARCH`, `GKEXPERIMENT`, `GKTOOLCHAIN`, `GKFLAGS`, `GKPROXY`, …), and a new `GKHOME` variable locates gecko's user configuration directory, so gecko recognizes its own config without depending on Go's variables.

**Descrição / Description:**

- New `GKHOME` environment variable (defaults to `<user-config-dir>/gecko`). It can only be set through the OS environment (like `GKENV`). The gecko environment configuration file defaults to `$GKHOME/env` instead of Go's `go/env`.
- Renamed environment variables across the whole tree: `GO111MODULE→GK111MODULE`, `GO_EXTLINK_ENABLED→GK_EXTLINK_ENABLED`, `GO_BUILDER_NAME→GK_BUILDER_NAME`, `GO_GCFLAGS→GK_GCFLAGS`, `GO_LDFLAGS→GK_LDFLAGS`, `GO_SSAFLAGS→GK_SSAFLAGS`, `GOROOT_BOOTSTRAP→GKROOT_BOOTSTRAP`, `GOROOT_FINAL→GKROOT_FINAL`, `GOCACHEPROG→GKCACHEPROG`, `GOAUTH→GKAUTH`, `GOBIN→GKBIN`, `GOCACHE→GKCACHE`, `GOENV→GKENV`, `GOEXE→GKEXE`, `GOEXPERIMENT→GKEXPERIMENT`, `GOFIPS140→GKFIPS140`, `GOFLAGS→GKFLAGS`, `GOGCCFLAGS→GKGCCFLAGS`, `GOHOSTARCH→GKHOSTARCH`, `GOHOSTOS→GKHOSTOS`, `GOINSECURE→GKINSECURE`, `GOMODCACHE→GKMODCACHE`, `GONOPROXY→GKNOPROXY`, `GONOSUMDB→GKNOSUMDB`, `GOPACKAGESDRIVER→GKPACKAGESDRIVER`, `GOPATH→GKPATH`, `GOPRIVATE→GKPRIVATE`, `GOPROXY→GKPROXY`, `GOSUMDB→GKSUMDB`, `GOTMPDIR→GKTMPDIR`, `GOTOOLCHAIN→GKTOOLCHAIN`, `GOTOOLDIR→GKTOOLDIR`, `GOVCS→GKVCS`, `GOWASM→GKWASM`, `GOWORK→GKWORK`, `GO_LDSO→GK_LDSO`, plus the architecture knobs `GO386→GK386`, `GOAMD64→GKAMD64`, `GOARM→GKARM`, `GOARM64→GKARM64`, `GOMIPS→GKMIPS`, `GOMIPS64→GKMIPS64`, `GOPPC64→GKPPC64`, `GORISCV64→GKRISCV64`.
- Runtime knobs (`GODEBUG`, `GOMAXPROCS`, `GOGC`, `GOMEMLIMIT`, `GOTRACEBACK`), compiler debug helpers (`GOSSAFUNC`, `GOSSADIR`, `GOSSAHASH`, …), test internals (`GO_TEST_*`, `GOCOVERDIR`, …) and the `GOOS_`/`GOARCH_` C-preprocessor macros are intentionally unchanged; `runtime.GOOS`/`runtime.GOARCH`, `build.Context`, `runtime.GOROOT()` and other Go API names are untouched.
- Default locations follow the new names: `GKPATH` defaults to `<home>/gecko`, `GKCACHE` to `<cache>/gecko-build`, and the GOROOT defaults file is `$GOROOT/gecko.env` (renamed from `go.env`) with `GK` keys (`GKPROXY`, `GKSUMDB`, `GKTOOLCHAIN`). `gecko env -w` now writes `$GKHOME/env`, `gecko env GKHOME` prints it, and `GKHOME`/`GKENV` are non-settable except via the OS environment.
- `cmd/dist`, `make.bash`/`make.bat`/`make.rc` and the `script` test framework condition names follow the rename (`[GKOS:linux]`, etc.). Where dist or the shell bootstrap scripts drive a standard Go bootstrap toolchain, both the `GO*` names (read by the standard Go binary) and the `GK*` equivalents are set.
- The `gecko install` command is used again internally by `cmd/dist` to build the toolchain (its removal broke `make.bash`); `gecko get` remains removed.

**Hash do commit / Commit hash:** `11f542c2`

**Mensagem do commit / Commit message:**
```
gecko: use GK* env vars for toolchain config, confine modules to internal
```

### Entry: fix fork fallout in the toolchain's own test suite

**Título / Title:** the toolchain's own tests no longer pin the `bin/go` → `bin/gecko` renaming or the gecko dialect restrictions to stale upstream text, so `go/build`, `cmd/cover` and `internal/testenv` pass again.

**Descrição / Description:**

- `go/build`: `TestImportPackageOutsideModule` now expects `no gecko.json found in current directory or any parent directory` (was `go.mod file not found ...`), and the `TestVendorPackages` allowlist gains the `cmd/vendor` dependency prefixes used by the fork's `cmd/gpm` package manager (`github.com/charmbracelet`, `github.com/atotto/clipboard`, `github.com/aymanbagabas`, `github.com/catppuccin`, `github.com/clipperhouse`, `github.com/dustin/go-humanize`, `github.com/erikgeiser`, `github.com/lucasb-eyer`, `github.com/mattn`, `github.com/mitchellh`, `github.com/muesli`, `github.com/rivo/uniseg`, `github.com/sahilm`, `github.com/xo/terminfo`).
- `cmd/cover` `TestCover`: the instrumented output is compiled as a package in a throwaway module (`go run .` in a temp directory with a `go.mod`) instead of naming `.go` files on the command line, which gecko rejects.
- `cmd/go` `gopath_install` script + `internal/work/build.go`: the `helloworld` fixture and the CLI message now use `.gk` (`gecko: no install location for .gk files listed on command line (GOBIN not set)`).
- The `testplugin` fixtures and the `goroot_executable`/`goroot_executable_trimpath` check programs remain in upstream Go syntax and continue to fail by design — gecko cannot compile `.go` files, whether named on the command line or contained in a package, and converting those fixtures is out of scope (see the gecko dialect rules below).

**Hash do commit / Commit hash:** `0bce5daa`

**Mensagem do commit / Commit message:**
```
gecko: fix fork fallout in the toolchain's own tests
```

### Entry: bin/gecko command

**Título / Title:** the build now installs the toolchain command as `bin/gecko` only. `make.bash` no longer creates `bin/go`, and the in-tree code that located the command at `$GOROOT/bin/go` now uses `$GOROOT/bin/gecko`.

**Descrição / Description:**

- `cmd/dist/build.go`: `gorootBinGo` points at `$GOROOT/bin/gecko`; after `go install cmd/go` the `goInstall` helper renames the freshly written `$GOROOT/bin/go` to `$GOROOT/bin/gecko` (`moveGoBinToGecko`), and the old post-bootstrap copy of `go` to `gecko` was removed. `checkNotStale` skips `cmd/go`, which `go list` always reports as "not installed" because its on-disk name is now `gecko`.
- Runtime references switched to `gecko`: `go/build`, `go/internal/srcimporter`, `internal/exportdata`, `internal/testenv` (prefers `gecko`, falls back to `go`), `internal/trace/traceviewer`, `cmd/compile/internal/testimporter`, `cmd/cover`, `cmd/go/internal/{bug,doc,work}`, `cmd/internal/script/scripttest`, and `syscall/mksyscall_windows.go`. The shell scripts (`all.bash`, `clean.bash`, `run.bash`, `buildall.bash`, `bootstrap.bash`, `all.rc`, `run.rc`) and `cmd/distpack` now name `bin/gecko`.
- Tests updated to the new name: `internal/testenv`, `cmd/api`, `cmd/cgo/internal/{testplugin,testshared}`, `cmd/internal/{bootstrap_test,moddeps}`, `os/exec`, `runtime/race`, `cmd/go/internal/cfg`, plus the `bug`, `cover_switch_toolchain`, `generate_goroot_PATH`, `mod_doc_path`, `tool_build_as_needed`, `run_goroot_PATH` and `test_goroot_PATH` scripts.
- Downloaded toolchains (`cmd/go/internal/toolchain/select.go`) and the bootstrap parser (`cmd/dist/buildtool.go`, `$GOROOT_BOOTSTRAP/bin/go`) intentionally keep `bin/go`.

**Hash do commit / Commit hash:** `3ac180dc`

**Mensagem do commit / Commit message:**
```
gecko: install the toolchain command as bin/gecko
```

### Entry: async/await

**Título / Title:** `.gk` files gain `async func` and `await`. An async function returns a *future* — a buffered channel of its declared result, or `chan any` when it declares none — and runs its body on its own goroutine (started immediately, like `go`). `await expr` blocks until the value is ready and yields it, so `await f(x)` and a stored future (`fut = f(x)` then `await fut`) both work. An async function may declare at most one result, must have a body, and a class `constructor` cannot be async.

**Descrição / Description:**

- Syntax (`cmd/compile/internal/syntax`): `tokens.go` gains `_Async`/`_Await` after `_Throw` (`token_string.go` updated); `scanner.go` recognizes the two keywords only in `.gk` files and does not insert a semicolon after them. `parser.go` parses `await e` as a unary expression and **desugars async functions and await at parse time**:
  ```go
  async func f(a: int): T { <body> }

  // becomes
  func f(a: int) chan T {
      var geckoAsyncCh: chan T = make(chan T, 1)
      go func() { geckoAsyncCh <- func() T { <body> }() }()
      return geckoAsyncCh
  }
  ```
  `await e` becomes the receive `<-e`. A function with no declared result runs its body for its side effects and sends `null` (so awaiting it yields `any`). The inner closure keeps the declared result type, so `return` statements are checked against the declaration and no special control-flow handling is needed (unlike try/catch, returns do not cross the goroutine boundary). `async` is accepted as a top-level declaration (`async func`), after `export`, and as a class method; `async constructor` is rejected, as is an async function with more than one result or without a body.
- No IR changes: the desugared AST is ordinary `go`/`chan`/`make`/receive code, so noder/typecheck/walk are untouched on the compiler path.
- Formatter mirror (`go/token`, `go/scanner`, `go/ast`, `go/parser`, `go/printer`): new `ASYNC`/`AWAIT` tokens, `ast.AwaitExpr` (with `Walk` support) and `FuncDecl.Async`/`AsyncPos`, parser support for async functions and class methods plus the `await` unary operator, and canonical printing (`async func`, `await expr`, `async method`). `go/types` (partial gecko mirror) models an async function object as returning `chan T` while checking its body against the declared result, and typechecks `await` as a receive.
- Tests/examples: `testdata/local/gecko_async.gk` covers async result/void functions, futures used as channels, `await` of a non-future, and `await` as a statement; the `go/printer` `gecko.gk` golden covers parsing/formatting; new `examples/11_async.gk` demonstrates the feature.

**Hash do commit / Commit hash:** `2e187f665b`

**Mensagem do commit / Commit message:**
```
gecko: add async/await
```

### Entry: cross-package class members, gecko.json projects, and gecko.json-first errors

**Título / Title:** struct fields and methods of gecko classes are now reachable from importing packages regardless of capitalization (e.g. `p.name` / `p.greet()` on a value returned by `new pkg.Class(...)`), `examples/` is now a `gecko.json` project instead of a Go module, and the "no main module" error asks for a `gecko.json` (`gpm init`) rather than a `go.mod`.

**Descrição / Description:**

- Visibility (`cmd/compile/internal/types2/object.go`): `Exported()` reports a struct field or method of a gecko package as exported regardless of capitalization. Fields and methods are the only objects without a parent scope; package-level declarations keep the explicit `export` override set by the resolver (so `export func greet` is public while a bare `func PrivateGreeting` stays private). This is what makes a qualified `new pkg.Class(args)` usable across packages. The `go/types` mirror is regenerated, and the generator (`go/types/generate_test.go`) now rewrites `IsGecko` to `_IsGecko` for `object.go`.
- Examples layout: new `examples/gecko.json` turns the examples directory into a gecko project, so `bin/go` stays in GOPATH-style project mode and never asks for a `go.mod`. `examples/05_exports.gk` imports `"exportlib"` (resolved from `<root>/exportlib` by `geckoLookupProjectPackage`) instead of the former module path `gecko.example/exportlib`.
- go command (`cmd/go/internal/modload/init.go`): `noMainModulesError` now reads "no gecko.json found in current directory or any parent directory; run 'gpm init' to create a gecko project" instead of pointing at `go.mod`, so building gecko sources gives a gecko-oriented fix.

**Hash do commit / Commit hash:** `1a478992aa`

**Mensagem do commit / Commit message:**
```
gecko: expose class members across packages and use gecko.json projects
```

### Entry: try/catch/finally and throw

**Título / Title:** `.gk` files gain JavaScript-style error handling: `throw expr` raises an error and `try { ... } catch (e) { ... } finally { ... }` handles it. The catch may bind a variable (`catch (e)`, of type `any`) or omit it (`catch { ... }`), and either clause is optional as long as one is present (`try`/`finally` without `catch` is allowed). `return`, `goto`, and any `break`/`continue` that would leave the try/catch/finally body are rejected with a clear compile error.

**Descrição / Description:**

- Syntax (`cmd/compile/internal/syntax`): `tokens.go` gains `_Try`/`_Catch`/`_Finally`/`_Throw` and `token_string.go` is regenerated; `scanner.go` recognizes the four keywords only for `.gk` files (they stay identifiers in `.go`). `parser.go` parses the statement and **desugars it at parse time** into an immediately-invoked function literal, because a panic unwinds the stack and `recover` only has effect inside a deferred function:
  ```go
  func() {
      defer func() { <finally body> }()
      defer func() {
          e = recover()
          if (e != null) { <catch body> }
      }()
      <try body>
  }()
  ```
  Defers run LIFO, so the catch runs first (recovering the panic) and the finally runs afterwards on every path, including when the catch itself throws. `throw x` becomes `panic(x)`. The catch variable is declared by the `e = recover()` assignment inside the deferred closure, so no identifier renaming is needed; an omitted or `_` binding uses the internal name `geckoTryErr`. `checkGeckoTryControl` walks the bodies (stopping at nested function literals) and rejects a `return`/`goto` or a `break`/`continue` that is not enclosed by a loop/switch/select inside the body.
- No IR changes: the desugared AST is ordinary Go, so noder/typecheck/walk are untouched on the compiler path.
- Formatter mirror (`go/token`, `go/scanner`, `go/ast`, `go/parser`, `go/printer`): new `TRY`/`CATCH`/`FINALLY`/`THROW` tokens, `ast.TryStmt`/`ast.CatchClause`/`ast.ThrowStmt` nodes (with `Walk` support), parser support, and canonical printing (`try { ... } catch (e) { ... } finally { ... }`). Unlike the compiler parser, the mirror keeps the explicit AST so formatting round-trips faithfully. `go/types` (partial gecko mirror) registers the catch variable with type `any` and typechecks `throw` like `panic`.
- Tests/examples: `testdata/local/gecko_trycatch.gk` covers recovery, try/finally without catch, bare catch, the `any` catch-variable type, and the parse-time `return` rejection; `go/parser` `TestGeckoDecls` and the `go/printer` `gecko.gk` golden cover parsing/formatting; new `examples/10_trycatch.gk` demonstrates the feature.

**Hash do commit / Commit hash:** `0ecfcc7a7e`

**Mensagem do commit / Commit message:**
```
gecko: add try/catch/finally and throw
```

### Entry: class inheritance with extends and super

**Título / Title:** classes can now inherit from another class with `class Dog extends Animal { ... }`, and inside a derived class `super` refers to the embedded base: `super(args)` calls the base constructor, `super.metodo(args)` calls a base method, and `super.campo` reads a base field. The base is embedded by value, so its fields and methods are promoted onto the derived class; like Go embedding there is no virtual override, and promoted base methods stay bound to the base implementation.

**Descrição / Description:**

- Syntax (`cmd/compile/internal/syntax`, `go/ast`, `go/parser`): `ClassType`/`ClassDecl` gained a `Base Expr` field; `extends <TypeName>` is parsed after the class name, and the class/extends parentheses are now optional (the old `class Greeter()` form still parses). The formatter (`cmd/compile/internal/syntax/printer.go`, `go/printer/nodes.go`) prints the canonical parenthesis-less `class Name extends Base` and round-trips `super` unchanged in `go/parser`.
- Semantics (`cmd/compile/internal/types2/class.go`): a derived class is a struct with an embedded base field (named after the base class, `Embedded: true`) followed by its own fields; base fields (direct and promoted) and the base type name are filtered from the derived field walk, and the embedded field carries the gecko export flag so qualified bases remain visible across packages. A qualified `extends pkg.Class` marks the import as used.
- Desugaring (`cmd/compile/internal/syntax/parser.go`): `super(...)`, `super.m(...)` and `super.f` are rewritten to `this.<Base>.constructor(...)`, `this.<Base>.m(...)` and `this.<Base>.f` before the class is lowered, so no changes were needed in the noder (unified IR).
- Tests/examples: `testdata/local/gecko_inherit.gk` covers promotion, `super`, undefined/non-class/cyclic bases; `go/parser` `TestGeckoDecls` and the `go/printer` `gecko.gk` golden cover parsing/formatting; `examples/04_classes.gk` gained Animal/Dog/Puppy.

**Hash do commit / Commit hash:** `0ecfcc7a7e`

**Mensagem do commit / Commit message:**
```
gecko: add class inheritance via extends and super
```

### Entry: input example rewritten and renamed to 09_input.gk

**Título / Title:** `examples/09_commit.gk` was replaced by `examples/09_input.gk`, a focused tour of the `input` builtin instead of a commit-message validator.

**Descrição / Description:**

- The new example demonstrates the different ways to read a line: a prompt-less `input()`, a plain string prompt, a multi-argument prompt (joined with spaces like `println`), and a `${...}` template prompt.
- It shows that `input` returns the line *including* its trailing `\n` (CRLF normalized to LF) and returns `""` at EOF, so it pairs a small `chomp` helper with a `for` loop that reads until a blank line or EOF.
- `examples/09_commit.gk` was deleted; no test references the old name.

**Hash do commit / Commit hash:** `0ecfcc7a7e`

**Mensagem do commit / Commit message:**
```
docs(examples): replace 09_commit with an input tour (09_input.gk)
```

### Entry: user-facing messages no longer mention the go command

**Título / Title:** every message shown to the user now refers to gecko commands and the `gecko:` error prefix, never to `go`: help and usage texts, error prefixes, and suggested commands (`gecko mod tidy`, `gecko mod init`, `gecko mod download`, `gecko mod vendor`, `gecko get`, `gecko work use`, `gecko env -w`, `gecko install`, and so on).

**Descrição / Description:**

- `cmd/go` error prefixes, help pages and usage lines already used `gecko`; the remaining spots that still printed `go <verb>` in user-facing messages were switched across `cmd/go/internal/{work,doc,modload,modfetch,envcmd,fips140}` and the `cmd/{vet,compile,cover,trace,nm,dist}` tools; `cmd/go/alldocs.go` was regenerated with `TestDocsUpToDate -fixdocs`.
- `go bug` keeps the truthful labels `GOROOT/bin/go ...` (the probe literally executes `$GOROOT/bin/go`), so the `bug` script test passes again.
- About 50 `testdata/script` expected-output strings (`stderr`/`stdout`) were updated, leaving script-runner invocations (`go ...`) and intentional `go` references (`.go` extensions, `go.mod`/`go.work`, `go 1.x` versions, `GOROOT/bin/go`) untouched.
- Verified with the full `go test ./cmd/go` suite: no new failures versus the pristine tree, and one previously failing script (`mod_gobuild_import`) now passes.

**Hash do commit / Commit hash:** `f8d44e0ccf`

**Mensagem do commit / Commit message:**
```
all: brand user-facing messages as gecko
```

### Entry: gecko terminology calls concurrent execution units "tasks"

**Título / Title:** gecko now refers to its concurrent execution units as **tasks** instead of goroutines. The `go` keyword and its semantics are unchanged: `go minhaFunc()` still launches one task. A repo-wide mechanical rename updated every comment and documentation reference from "goroutine(s)" to "task(s)", preserving case and pluralization, while all identifiers, strings, vendored code and test-data binaries were left untouched.

**Descrição / Description:**

- Comments in Go sources (`.go`/`.go2`) were rewritten token-by-token so only comment text changed; identifiers like `runtime.NumGoroutine`, `GoroutineHandlerFunc` and `c.goroutine` and all string literals are preserved.
- Documentation files (`.md`, `.html`, `.txt`) and the `go/doc` golden outputs were renamed in lockstep so golden-based tests keep passing; `CHANGELOG.md`, vendored trees, third-party race runtime and binary test data (`.test`, `.json`) were excluded.
- Renamed forms: `goroutine`→`task`, `goroutines`→`tasks`, `Goroutine`→`Task`, `Goroutines`→`Tasks`; 527 files, 2796/2796 lines.
- Verified with `go build std` and the `go/doc`, `go/parser`, `cmd/compile/internal/syntax`, `go/printer`, `go/format`, `cmd/fmt` and `cmd/trace` test suites.

**Hash do commit / Commit hash:** `1caceb9d9d`

**Mensagem do commit / Commit message:**
```
all: rename goroutine to task in comments and docs
```

### Entry: modules.lock created only on install

**Título / Title:** `gpm init` no longer writes a `modules.lock` file: a fresh project only contains `gecko.json` and `main.gk`. The lock file appears when the first module is installed (`gpm install <package>`), and a `gpm install`/`gpm remove` that leaves the dependency tree empty drops any stale lock, so a project with no installed modules only carries `gecko.json`.

**Descrição / Description:**

- `src/cmd/gpm/internal/init/init.go`: removed the unconditional `lockfile.Save` call from `Create`.
- `src/cmd/gpm/main.go`: `persist` now returns a `saved` bool and writes `modules.lock` only when the resolved tree is non-empty, removing it otherwise; `cmdInstall` reports "saved gecko.json" when nothing was locked; `printInitSummary` no longer lists `modules.lock`.
- `src/cmd/gpm/internal/init/init_test.go`: asserts `modules.lock` is absent after init.

**Hash do commit / Commit hash:** `bb8cdc54b8`

**Mensagem do commit / Commit message:**
```
gpm: only create modules.lock when a module is installed
```

### Entry: typed mode for the gecko.json type field

**Título / Title:** the `type` field of `gecko.json` now controls how strictly the gecko toolchain type-checks `.gk` code. With `"type": "typed"` the compiler forces explicit type annotations and explicit `&`/`*` operators, TypeScript-style; with `"type": "dynamic"` (the default) every dynamic-typing convenience keeps working. `gpm init` gained `-t/--type`, `-d` and `-T` flags plus a wizard step to pick the mode.

**Descrição / Description:**

- `src/cmd/compile/internal/types2/api.go` + `src/go/types/api.go`: new `Config.GeckoTyped` field; `geckoDynamicFile(pos)` (helpers in `gecko.go` of both packages) reports whether the dynamic features apply at a position — always in dynamic mode, never for non-gecko files.
- `src/cmd/compile/internal/types2/{literals,stmt,range,operand}.go` + `src/go/types/{literals,stmt,range,operand}.go`: in typed mode the following are disabled: type-inferred composite literals (`var a = [1, 2, 3]` → `missing type in composite literal`), declare-or-assign (`x = e` and `for (i = range x)` → `undefined: x`), and implicit pointer address-of/dereference assignments (`var p: *int = v` / `var d: int = p` → they now require explicit `&`/`*`). Literals with an explicit context type (`var xs: []int = [1, 2, 3]`) and Go-standard inference (`var x = 5`) remain valid.
- `src/cmd/compile/internal/noder/irgen.go`: wires `conf.GeckoTyped` from the new `-geckotype` flag (`src/cmd/compile/internal/base/flag.go`, registered in `src/cmd/compile/internal/gc/main.go`).
- `src/cmd/go/internal/load/gecko.go`: new `GeckoProjectType` reads the project root `gecko.json`; `src/cmd/go/internal/work/gc.go` appends `-geckotype=typed` when the project declares it.
- `src/cmd/gpm/main.go` + `src/cmd/gpm/internal/ui/wizard.go`: `gpm init` mode selection (`-t/--type <dynamic|typed>`, `-d`, `-T`) and a new wizard step, validated by `manifest.ValidType`.
- `src/go/types/check_test.go`: `TestGeckoTypeMode` covering both modes.

**Hash do commit / Commit hash:** `cc0d34dfe3`

**Mensagem do commit / Commit message:**
```
feat: typed mode forcing explicit type annotations in gecko
```

### Entry: implicit pointer address-of and dereference

**Título / Title:** gecko now applies `&` and `*` internally. In a `.gk` file a value of type `E` is assignable to a variable of type `*E` (implicit address-of, for addressable values), and a value of type `*E` is assignable to a variable of type `E` (implicit dereference), so users write `a = b` instead of `a = *b`, and `b = a` instead of `b = &a`. Explicit `&` and `*` still work, as does `null`; taking the address of a non-addressable value is a compile error.

**Descrição / Description:**

- `src/cmd/compile/internal/types2/operand.go` + `go/types/operand.go` (`assignableTo`): new gecko rule accepting single-level `E`↔`*E` mismatches (gated on `.gk` files); the address-of direction requires the source to be addressable (`variable` mode), otherwise it reports `cannot take the address of a non-addressable value`.
- `src/cmd/compile/internal/noder/writer.go` (`implicitConvExpr`): for gecko files, when the pointer pattern matches, the expression is wrapped in an implicit unary node (`OADDR` or `ODEREF`) instead of an asserted conversion — this covers assignments, call arguments, returns, and composite literal fields. The multi-value (`N:1`) path of `multiExpr` emits an address-of/dereference "fix" per element next to the existing conversion flag.
- `src/cmd/compile/internal/noder/reader.go`: decodes the multi-value pointer fix by wrapping the element temporary in an implicit `&`/`*`.
- `src/go/types/check_test.go`: new `TestGeckoImplicitPointers` checker test.

**Hash do commit / Commit hash:** `b9af1082b5`

**Mensagem do commit / Commit message:**
```
feat: implicit pointer address-of and dereference in gecko
```

### Entry: gpm module resolution in the gecko toolchain

**Título / Title:** gecko now resolves `import` paths against the project's gpm module cache, so `gpm i` modules compile straight from `gecko run`. Both the full repository path (`github.com/fatih/color`) and the short module name (`color`) resolve to the installed snapshot; imports that live under an installed module (`golang.org/x/sys/unix`) resolve into it, and Go modules are also importable by their own `module` path from `go.mod` (`golang.org/x/sys` → the `github.com/golang/sys` snapshot). gpm additionally installs a Go module's transitive dependencies by reading its `go.mod` (`require`s), so real Go libraries like fatih/color are self-contained.

**Descrição / Description:**

- `src/cmd/go/internal/load/gecko.go`: `geckoLookupGPM` scans `<root>/gpm/modules` (or `$GPM_HOME/modules`) for `<repository>@<version>` snapshot dirs, keys them by full repository path, by short name (`github.com/fatih/color` → `color`), by `go.mod` module path and by subpackage prefix, then `ImportDir`s the best match. Called as a fallback from `geckoLookupProjectPackage`; `gpmModuleRoot` mirrors gpm's `storage.DefaultHome`.
- `src/cmd/gpm/internal/source/source.go` + `github.go`: new `GoManifest` interface method parses a module's `go.mod` `require` list into the manifest dependency shape, mapping `github.com/*` directly and `golang.org/x/*` to `github.com/golang/*`.
- `src/cmd/gpm/internal/resolver/resolver.go`: when a module has no `gecko.json`, the resolver falls back to `GoManifest`, so transitives install recursively and land in `modules.lock`.
- `src/cmd/gpm/internal/resolver/resolver_test.go`: fake source implements `GoManifest`.

**Hash do commit / Commit hash:** `9e7dccc39c`

**Mensagem do commit / Commit message:**
```
feat: resolve gpm-installed modules and their Go dependencies as imports
```

### Entry: command-line executable name for .gk/.go files

**Título / Title:** building an explicit list of source files names the executable after the first file regardless of extension. The previous `.gk`-only `TrimSuffix` left `.go` files with their extension (`output.go`), which broke `go build output.go` naming and the upstream loader test.

**Descrição / Description:**

- `src/cmd/go/internal/load/pkg.go` (`exeFromFiles`): strip the file's own extension (`filepath.Ext`) instead of a hard-coded `.gk`, so both `util.gk` and `output.go` yield `util` / `output`.

**Hash do commit / Commit hash:** `60cd95fc76`

**Mensagem do commit / Commit message:**
```
fix: derive command-line executable name from the source extension
```

### Entry: implicit byte conversions in binary operations

**Título / Title:** gecko now applies implicit byte conversions in binary operations: `string + byte` (and `byte + string`) concatenate the byte as its character, so `cur += s[i]` behaves as `cur += string(s[i])`; in arithmetic and comparisons a byte promotes to a wider numeric type (`byte + byte` → `int`, `byte + int` → `int`, `int + byte` → `int`, `float64 + byte` → `float64`, `c - '0'` → `int`). Direct assignment still requires compatibility (`b += 5` with `b: byte` is a compile error), and explicit casts (`int(c)`, `string(c)`) keep working.

**Descrição / Description:**

- `src/cmd/compile/internal/types2/gecko.go`: new exported `GeckoBinaryCommonType(xt, yt, op)` — the single source of truth for when a coercion applies and to which common type — plus `geckoCoerceBinary`, which mutates the operand types in place (and folds integer constants into a character string for constant concatenation). `isByteType` matches any uint8-underlying type.
- `src/cmd/compile/internal/types2/expr.go` (`binary`): after `matchTypes`, untyped operands and mixed byte/string binary ops are run through `geckoCoerceBinary` before the mismatch check; comparisons are coerced the same way.
- `src/cmd/compile/internal/noder/writer.go`: the `*syntax.Operation` case consults `types2.GeckoBinaryCommonType` on the recorded operand types and, when a gecko coercion applies, forces explicit conversions with the new `geckoConvert` helper (`convertExpr` with `implicit=false`, since `byte→int`/`byte→string` are not Go-assignable). The `x op= y` assign-op case similarly coerces the RHS (e.g. `cur += c`, `n += b`).
- Unchanged behavior: `string + int` (typed) still reports mismatched types; only byte and untyped integer constants concatenate as characters.

**Hash do commit / Commit hash:** `90ce1d3ec5`

**Mensagem do commit / Commit message:**
```
feat: implicit byte conversions in gecko binary operations
```

### Entry: var/const declarations and optional constructor in class body

**Título / Title:** `var x = 1`, `const X = 3`, or the bare shorthand `x = 1` are now allowed directly in the class body; field initializers run before the constructor body, and a class with fields but no explicit constructor gets a synthetic one. Const class fields are read-only: reassignment via `this.X = ...` is a compile error.

**Descrição / Description:**

- `src/cmd/compile/internal/syntax/nodes.go`: `ClassType` gained `Fields []*VarDecl`, `Consts []*ConstDecl`, and `Inits []Stmt`; `FuncDecl` gained `GeckoSynthCtor bool`.
- `src/cmd/compile/internal/syntax/parser.go`: `classDecl` body loop now handles `_Var`, `_Const`, and bare `_Name` field branches; `fieldInitStmts` desugars each declaration into `this.x = v` assign statements; after the body loop, if `Inits` is non-empty, the parser either prepends inits to the explicit constructor body or synthesizes a `constructor()` marked `GeckoSynthCtor`. Removed the now-dead `classMethod()` in favor of `classMethodTail`.
- `src/cmd/compile/internal/syntax/printer.go`: `ClassType` case prints fields, consts, and methods in source order, skipping synthetic ctors; a `classInits` field drives `classMethodBodyPruned` to drop the merged init prefix from a real constructor body.
- `src/cmd/compile/internal/types2/class.go`: `geckoClassType` now records `cls.Fields` and `cls.Consts`, supports explicit type annotations via the `declType` map, and populates `geckoConstFields` / `geckoConstRhs` on the `Checker` to track read-only fields.
- `src/cmd/compile/internal/types2/assignments.go` + `check.go`: `assignVar` calls `geckoConstFieldGuard`, which rejects `this.X = e` for const fields unless the RHS node is the parser-generated initializer (node identity check).
- `src/go/ast/ast.go`: `ClassDecl` gained `Fields`, `Consts`, `Inits`; `FuncDecl` gained `GeckoSynthCtor`. `ClassDecl.End()` considers fields/consts.
- `src/go/parser/parser.go`: `parseClassDecl` handles `var`/`const` and bare field branches, merges inits, synthesizes ctor; new `parseClassMethodTail` and `fieldInitStmts` helpers.
- `src/go/printer/nodes.go`: `classDecl` prints fields/consts/methods in source order, prunes ctor init prefix via `classMethodBodyPruned`; new `classField` wrapper implements `ast.Node`.

**Hash do commit / Commit hash:** `3565386f83`

**Mensagem do commit / Commit message:**
```
feat: support var/const declarations and optional constructor in class body
```

### Entry: input() accepts multiple arguments and reads until EOF

**Título / Title:** `input()` now accepts one or more arguments, formatting the prompt identically to println (space-joined, no trailing newline), and then drains stdin until EOF, returning the entire accumulated content (all lines). Example: `input("name:", "default")` prints `name: default` then reads everything that comes in; piped input is consumed fully in a single call.

**Descrição / Description:**

- `src/runtime/geckoinput.go`: prompt is built via `geckoAppendAny` per arg with space separators (no trailing newline); the read loop now drains stdin until EOF instead of stopping at the first newline, only wrapping `geckoinputBuf` when full; CRLF is normalized to LF. Interactive users signal end of input with EOF (Ctrl-D).
- `src/cmd/compile/internal/typecheck/_builtin/runtime.go`: runtime signature updated to `func geckoinput(...any) string`.
- `src/cmd/compile/internal/typecheck/builtin.go`: `typs[34]` is now `func(...any) string` (tag 82); `inParams` is a variadic `[]any`.
- `src/cmd/compile/internal/typecheck/func.go`: `tcInput` iterates over all arguments (like `tcPrint`), removing the fixed-arity `errors` import.
- `src/cmd/compile/internal/types2/universe.go`: `_Input` arity changed from `{1, false}` to `{0, true}` (any count).
- `src/cmd/compile/internal/types2/builtins.go`: `_Input` case loops over arguments (mirroring `_Print`), calls `check.assignment` per arg, and records the builtin signature.
- `src/cmd/compile/internal/walk/builtin.go`: `walkInput` uses the reader pattern (`typecheck.Expr(call)`) to build the call, avoiding the variadic-arity `Fatalf` guard.

**Hash do commit / Commit hash:** `6304af82fd` (varargs prompt) + follow-up (read-until-EOF)

**Mensagem do commit / Commit message:**
```
feat: allow input() to accept multiple arguments like println
```

### Entry: gecko class field type inference covers bool, signed and class-instance literals

**Título / Title:** `if (t.done)` now typechecks whenever `done` is a real field, not only when it is a typed constructor parameter: class fields initialized with `true`/`false` literals infer `bool`, signed numeric literals (e.g. `-3`) infer their numeric type instead of `any`, `this.x = new SomeClass()` infers the class's pointer type (self-referential and nested, from the same or another package), and a `null` first assignment lets a later non-null assignment in the constructor decide the field type.

**Descrição / Description:**

- `src/cmd/compile/internal/types2/class.go` (`geckoFieldType`, `geckoClassType`): the field type switch gained `true`/`false` → `bool`, `null` → `any`, unary `+`/`-` over numeric literals → the literal's type, and `new Class(...)` → the class pointer type resolved structurally with `geckoNewType` (no declaration type-checking, so linked-list shapes do not recurse); the constructor scan now records a `null` assignment without letting it shadow a subsequent value assignment.
- `src/cmd/compile/internal/types2/testdata/local/gecko_dotcond.gk` (new): regression test exercising the whole dot-condition matrix — literal bools, negation, comparisons, signed numerics, nested/self class instances, `null`-then-value, `if`/`else if`/`switch (t.done)`. Covered by `TestLocal`.

**Hash do commit / Commit hash:** `1332bed6e1`

**Mensagem do commit / Commit message:**
```
fix(cmd/compile): infer gecko class field types from bool, signed and class-instance literals
```

### Entry: template strings become general-purpose expressions usable in any context

**Título / Title:** Template strings (`\`…${expr}…\``) are no longer limited to print/println arguments: they now work anywhere an expression is expected — variable initializers, return values, function arguments, composite values — and interpolate any value using print/println formatting semantics (strings, ints, bools, floats, slices, maps, and `null`-safe chains), compiled into a call to the runtime `geckotemplate` helper.

**Descrição / Description:**

- `src/cmd/compile/internal/noder` (`codes.go`, `writer.go`, `reader.go`): a new `exprTemplate` encoding represents a template as an ordered list of literal segments and expressions. Static segments are materialized as plain string constants; interpolated expressions are lowered (with null-safe chains routed through the existing `geckoNullChain` machinery) into arguments of a single synthetic call. The pre-existing print-context lowering (`exprPrintTemplate`, `ir.OPRINT` arity) is unchanged, so `print`/`println` templates keep their historical output.
- `src/cmd/compile/internal/typecheck` (`builtin.go`, `_builtin/runtime.go`): the runtime function `geckotemplate` is registered as builtin tag 166 with type `func(...any) string`.
- `src/runtime/geckoprint.go`: implements `geckotemplate` plus the `geckoAppendAny` formatter family (`geckoAppendSlice`, `geckoAppendGeneric`, `geckoAppendSliceGeneric`, `geckoAppendMapGeneric`, `geckoAppendHex`), rendering any value the way print/println do; a null-safe chain whose target is nil renders as `null`.
- This change also fixed a latent make.bash hang: `geckoprint.go` now uses `runtime/internal`-safe `internal/strconv` for integer/float formatting instead of the standard `strconv` package, removing the `runtime → strconv → errors → reflectlite → runtime` import cycle that stalled the bootstrap toolchain during `dist bootstrap`.

**Hash do commit / Commit hash:** `addb369df2`

**Mensagem do commit / Commit message:**
```
feat(cmd/compile): allow template strings in general expression contexts
```

### Entry: gecko projects no longer need a src/ directory for local imports

**Título / Title:** Local package imports can live right at a `gecko.json` project root: `import "utils"`, `import "gutil"`, or a single-file `import "utils"` for `<root>/utils.gk` all resolve without a `src/` tree. The `src/`-style layout keeps working for compatibility.

**Descrição / Description:**

- `src/cmd/go/internal/load/gecko.go`: new `geckoLookupProjectPackage` resolves a plain import path against the enclosing gecko project root, trying, in order, `<root>/src/P` (legacy), `<root>/P` (directory), and `<root>/P.gk` (single-file package via a scoped `ReadDir` so sibling root files stay out of it). Standard-library packages always win and are never remapped.
- `src/cmd/go/internal/load/pkg.go`: the helper runs as a fallback in `loadPackageData` when GOPATH-mode resolution fails (module mode is untouched). Combined with the existing `maybeExtendGOPATHForGecko`, main packages importing root-level packages now load, build (including correctly wired `importcfg` files), and link end-to-end.
- Verified on a `gecko.json` project with a root `main.gk` importing a root dir package (`gutil/`), a subdir package (`db/`), and a root single-file package (`utils.gk`).

**Hash do commit / Commit hash:** `c43899f5f3`

**Mensagem do commit / Commit message:**
```
feat(cmd/go): resolve local imports from src-less gecko project roots
```

### Entry: gecko fmt rewrites gecko.json projects end-to-end; the formatter binary is renamed from gofmt to fmt

**Título / Title:** `gecko fmt` no longer needs go.mod: in a `gecko.json` project a bare `gecko fmt` walks the whole tree and reformats every `.gk` (and `.go`) file with the toolchain's own formatter, whose binary was renamed `gofmt` → `fmt`. The formatter itself was extended so the full gecko surface round-trips faithfully.

**Descrição / Description:**

- `src/cmd/fmt` (renamed from `src/cmd/gofmt`, binary installed as `$GOROOT/bin/fmt`): `isGoFilename` now accepts `.gk`; `.gk` files are printed with the gecko printer modes (`GeckoParens | GeckoColons`); help/usage/telemetry strings and `cmd/dist`/`cmd/distpack` install lists updated (`binExesIncludedInDistpack`, `bin/` allowlist, distpack `binExes` and tests).
- `src/cmd/go/internal/fmtcmd/fmt.go`: outside module mode, `gecko fmt` resolves files by walking the `gecko.json` project tree (or explicit file/directory arguments) straight to the `fmt` binary, skipping `vendor/` and dot directories; package/pattern/error behavior (e.g. `gecko fmt does-not-exist`) still goes through the module loader so upstream reporting is unchanged. `gofmtPath` now looks up `fmt`.
- `src/go/parser`, `src/go/ast`, `src/go/printer`, `src/go/token`, `src/go/scanner`: full gecko syntax support for the formatter — `export` (`export { A, B }`, `export const/var/func` prefixes via `GenDecl.Export`/`FuncDecl.Export`), `class Name() { … }` (`ast.ClassDecl` with implicit-this methods), `new Type(args)` (`ast.NewExpr`), dynamic slices `[1, 2, 3]` (`CompositeLit.GeckoBrackets`), TypeScript-style imports `import Name, { a, b.x } from "path"` (`ImportSpec.TsNames/TsPos/TsFrom`), plus the `CLASS/NEW/EXPORT` tokens demoted back to identifiers in `.go` files.
- Tests: `TestGeckoDecls`, `cmd/fmt` idempotency over the tree incl. `examples/*.gk` (long-test now applies gecko modes to `.gk`), and new script test `fmt_gecko.txt`. `src/cmd/go/alldocs.go` regenerated.

Touched files: `src/go/{token/token.go,scanner/scanner.go,ast/{ast.go,walk.go,example_test.go},parser/{parser.go,parser_test.go},printer/nodes.go}`; `src/cmd/{dist/build.go,distpack/{pack.go,test.go}}`; `src/cmd/fmt/*` (moved from `src/cmd/gofmt`); `src/cmd/go/{internal/fmtcmd/fmt.go,internal/load/{pkg.go,gecko.go},internal/{toolchain/select.go,generate/generate.go,help/helpdoc.go},alldocs.go,testdata/script/{fmt_gecko.txt,fmt_load_errors.txt,gofmt_with_symlink.txt}}`.

**Hash do commit / Commit hash:** `c804352cd5`

**Mensagem do commit / Commit message:**
```
feat(cmd/fmt): rename gofmt to fmt and make gecko fmt walk gecko.json projects
```

### Entry: gecko drops the mod subcommand and any remaining go branding in the help

**Título / Title:** The `go` command's `mod` subcommand is removed from gecko: module maintenance (`go mod init/tidy/vendor/...`) no longer applies to go.mod-less `gecko.json` projects. The top-level `gecko help` output is now fully gecko-branded, no longer lists `mod`, and the "Additional help topics:" section is gone; `TestScript` skips the module-command scripts instead of failing.

**Descrição / Description:**

- `src/cmd/go/main.go`: `modcmd.CmdMod` and its import were dropped from the root command list, so `gecko mod ...` reports `gecko mod: unknown command`.
- `src/cmd/go/internal/help/help.go`: the "Additional help topics:" block was pruned from the top-level usage template (the topics remain reachable through `gecko help <topic>`).
- `src/cmd/go/internal/base/base.go` and the command `Short` strings (`env`, `generate`, `run`, `tool`, `version`) brand the surface as gecko ("Gecko is a tool for managing Gecko source code.", "print gecko version", "run specified gecko tool", ...).
- `src/cmd/go/alldocs.go` regenerated by `TestDocsUpToDate -fixdocs`.
- `src/cmd/go/script_test.go` (new `usesModCommand` helper): `TestScript` skips any script that invokes the removed `mod` subcommand — including `go help mod` and the `go -C <dir> mod ...` form — so the cmd/go script suite went from 345 failing scripts down to 117, the remainder being the intentional pre-existing `.gk`-extension failures.

Touched files: `src/cmd/go/{main.go,alldocs.go,script_test.go}`, `src/cmd/go/internal/{base/base.go,help/help.go,envcmd/env.go,generate/generate.go,run/run.go,tool/tool.go,version/version.go}`.

**Hash do commit / Commit hash:** `f14b7e0d27`

**Mensagem do commit / Commit message:**
```
refactor(cmd/go): remove the mod subcommand and prune the help listing
```

### Entry: gpm's TUI wizard and colorized CLI

**Título / Title:** The `gpm init` wizard is now a multi-step bubbletea form, and the rest of the CLI is colorized for TTY terminals, with the charmbracelet dependencies vendored into the `cmd` module where the toolchain build resolves them.

**Descrição / Description:**
`cmd/gpm/internal/ui/wizard.go` is rebuilt as a `charmbracelet/bubbletea` model (`wizardModel`) that walks the init questions as aligned `bubbles/textinput` fields with a progress bar and a summary screen, keeping the line-input and non-TTY fallbacks. Deps came in as `bubbles`, `bubbletea` and `lipgloss` plus their transitive closure, vendored into the **cmd** module (`src/cmd/go.mod` + `src/cmd/vendor`) — the GOROOT toolchain builds `cmd/gpm` in the `cmd` module during `make.bash`, so the earlier placement under the `std` module's `src/vendor` failed the bootstrap with `no required module provides package github.com/charmbracelet/...`.

CLI/UX changes alongside:
- `ui/colors.go` (new): ANSI `Bold`/`Dim`/`Red`/`Green`/`Yellow`/`Cyan`, a `Box` frame and `stripCodes` width math, honor `NO_COLOR` and are only enabled for TTY stdout (`EnableColorsForTTY`).
- `Terminal` (ui.go): prompts aligned on a 30-column label (`labelW`), `PromptP` with gray placeholder hints, `Errf` red validation lines, colored `>` selector and `[x]` checkbox marks.
- `main.go`: enables colors on TTY; manifest `dependencies`/`devDependencies` maps always exist after load (`.Defaults`); bare `"*"` specs are pinned to `^<locked version>` when persisting (`pinSpecs`); `gpm remove` clears the package from both `dependencies` and `devDependencies`; init summary and `gpm list`/lock tree output are colored.
- `manifest.go`: `Main` field added, `omitempty` tags, stable field order (`name, version, description, main, scripts, keywords, author, license, type`); `init.go` records `main`, `scripts` and `keywords`.

Tests: `wizard_test.go` (372 lines) and `main_test.go` cover the wizard steps and the CLI regression surface.

Touched files: `src/cmd/gpm/{main.go,internal/{init,manifest,ui}/*}`, `src/cmd/go.mod`, `src/cmd/vendor/*`.

**Hash do commit / Commit hash:** `ab518593c8`

**Mensagem do commit / Commit message:**
```
feat(cmd/gpm): rebuild the init wizard on bubbletea and colorize the CLI
```

### Entry: the go command speaks gecko

**Título / Title:** Every user-facing message of the `go` command is now branded `gecko` (prefix, version line, usage, help topics, toolchain and vet/workspace messages), while file names and module/toolchain syntax stay standard.

**Descrição / Description:**
The branding sweep re-writes the user-facing surface of `cmd/go` and its dependencies so a gecko user sees a coherent command: error prefix `gecko: ` (instead of `go: `), `gecko version go1.28-... X:simd`, `usage: gecko test`, `gecko help <topic>`, `gecko: upgrading toolchain...` / `gecko: using local toolchain...`, `gecko list -f cannot be used with -json`, `gecko vet -diff flag requires -fix`, `gecko mod why`, `gecko tool interrupter: signal: interrupt`, `see 'gecko help vcs'`, `For information about 'gecko mod tidy'`, `'gecko get' is no longer supported outside a module.`, and so on. Doc strings (`cmd/go/alldocs.go` is regenerated by `TestDocsUpToDate -fixdocs`) and help texts are branded too.

Two escapes from the string-literal sweep were caught by the test suite and fixed:
- `internal/buildcfg.Check` printed the invoking program's basename (`go: invalid GORISCV64...`); it now prints `gecko: ` when the driver is named `go`/`go.exe` (compile/link keep their own tool names).
- `cmd/go/internal/workcmd/use.go` used a backtick raw string that the quoted-string sweep could not see (`gecko: already added "..." as "..."`).

The gecko runtime routes legacy `print`/`println` writes to **stdout**; scripts and unit tests whose fixtures relied on `go run`/`go test` printing to stderr assert on `stdout` instead.

**Fork-only file rule (intentional, documented as such):** a `go` file named on the command line is rejected with `gecko does not compile .go files; use the .gk extension: <file>`. About 120 upstream `testdata/script` cases that pass `.go` files directly remain failing by design and are not converted (120 fail on the `.go` rule itself, plus `test_chatty_fail`, `test_benchmark_chatty_fail`, `test_fail_fast` and `cgo_bad_directives` whose scenarios are masked by it, and `build_static`, which needs static glibc unavailable in this Nix build). Renamed fixtures now live under `.gk` (`.go` → `.gk`, `:= ` → `var x: string` + `=`, colons on types).

Beyond the `.go` rule, the main divergences that the suite turned up were all in the sweep itself and have been fixed:
- `cmd/go/internal/modload/vendor.go` over-branded the `## ` annotation marker read from `vendor/modules.txt` (`go <ver>` → `gecko <ver>`). That broke the vendored deps language-version handling (`mod_vendor_goversion` fell back to the go1.16 guess, and `mod_goline_too_new` never fired the `requires go >=` check). The reader now keeps the upstream `go <ver>` format.
- `gover.go`, `toolchain/select.go` comments describing the go.mod `go <ver>` line were un-branded.
- Scripts whose programs print via `print`/`println` assert on `stdout` (gecko routes `print*` to stdout): `build_dash_x`, `build_link_x_import_path_escape`; `cover_list` dropped a vestigial `println(os.Args[1])` that polluted the compared build-id files.

Unit tests converted to `.gk`: `TestNoteReading` (note_test.go), `TestLinkerTmpDirIsDeleted`, `TestLdflagsArgumentsWithSpacesIssue3941` and `TestLdFlagsLongArgumentsIssue42295` (all green, incl. cgo via the `.gk`→`.go` bridge).

Touched files: `src/cmd/go/internal/*` (sweep), `src/cmd/go/alldocs.go`, `src/cmd/go/go_test.go`, `src/cmd/go/note_test.go`, `src/cmd/go/testdata/script/*.txt`, `src/internal/buildcfg/cfg.go`, `src/cmd/go/internal/workcmd/use.go`, `src/cmd/go/internal/load/gecko.go` (new).

**Hash do commit / Commit hash:** `0ba6a7e9c7` (branding sweep + gecko core plumbing), `b76a1da087` (unit tests → `.gk`), `d66482999b` (script README regenerated for the gecko dialect), `a03c0bb21b` (vendor annotations + remaining script divergences)

**Mensagem do commit / Commit message:**
```
refactor(cmd/go): brand all user-facing output as gecko
```

### Entry: TypeScript-style imports, ".gk opens" in `geckodays`, and a null-safe tour

**Título / Title:** `.gk` files accept TypeScript-style imports (`import x from "lib"`, `import { a, x.y } from "lib"`, and the mixed `import x, { a, b } from "lib"`), resolved by the compiler front end (types2) into the file scope; the go command's package loader understands the new forms too, so `go run`/`go build` work on `.gk` files that use them. Together with an in-tree project go.mod-less module (`gecko.json` + `main.gk` + `src/`), the new `geckodays` planner doubles as the runnable tour of gecko features.

**Descrição / Description:**
Gecko user files (`.gk`) gain three TypeScript-style import forms on top of the standard Go forms, all feature-gated on the `.gk` extension:

```gecko
import n from "fmt"              // default import: binds the exported symbol
                                 // n of "fmt" when present, otherwise "fmt"
import { Sprintf, any } from "fmt"   // named imports: bind exported symbols
import util, { firstWord, joinTags } from "utils"  // mixed form
import { math.abs } from "utils"  // qualified member: binds abs under the
                                  // synthetic package qualifier math
```

The compiler front end parses them in `cmd/compile/internal/syntax` (`importDecl` recognizes the `{ ... }` member list and the `from` keyword; new `TsFrom`/`TSMembers` fields on `ImportDecl` plus a new `TsMember` node; the printer round-trips them and the walker visits the member names). `types2` resolves them via `geckoTSImport` (new `cmd/compile/internal/types2/tsimport.go`): the default import binds the exported symbol when present (otherwise the package itself), plain members bind the exported symbols directly, and qualified members bind through a synthetic alias package; the import decl also records an implicit `PkgName`, and `Info.PkgNameOf` falls back to it, so the noder's declCollector (which checks `PkgNameOf(imp).Imported().Path()` for `embed`/`unsafe`) gets a non-nil package name for named-only and symbol-default imports. The `go/types` mirror regenerates and keeps the gecko helpers in `go/types/gecko.go` (hand-maintained, `literals.go` only carries the caller block).

The go command's package loader previously read the import section of a `.gk` file with Go-only logic, so `go run` failed in the load phase with `import path must be a string` (from `go/parser`) while `go tool compile` handled the same file fine. The textual `importReader` in `go/build` and `cmd/go/internal/modindex` now skips the TS tail up to the path literal, and `go/parser` accepts the TS forms for `.gk` files (`skipTSImportTail`, gated on the existing `p.gecko`). A related bug was fixed in the go.mod-less project resolution: `maybeExtendGOPATHForGecko` now prepends the project *root* to the build GOPATH (GOPATH mode already adds the inner `src/` when resolving `$GOPATH/src/<import>`), so `import ... from "utils"` in a project with `src/utils/utils.gk` resolves.

Also fixed en route: the syntax tree walker dereferenced a nil `TsMember.Qualifier`, and `geckoNullChain` in the noder regained method-call-result selections and gained `IndexExpr` terminal bases, so `tasks[len(tasks)-1].next.label()` prints `null` when `next` is null.

`geckodays` (new sibling workspace dir, not part of the Go repo) is the runnable feature tour: a single-file dynamic module (`gecko.json` `scripts.start` = `gecko run main.gk`, no go.mod) whose `main.gk` exercises the TS mixed import above, the SIMD fold (`simd.LoadUint8sPart`/`StorePart`, AVX/SVE), typed class constructors, template strings, byte indexing, and a null-safe ghost pointer call. Runs end to end via `gpm run start`.

Touched files: `src/cmd/compile/internal/syntax/{nodes,parser,printer,walk}.go`, `src/cmd/compile/internal/types2/{api,gecko,literals,resolver,tsimport}.go`, `src/cmd/compile/internal/noder/writer.go`, `src/go/types/{gecko,literals}.go`, `src/go/build/read.go`, `src/go/parser/parser.go`, `src/cmd/go/internal/load/gecko.go`, `src/cmd/go/internal/modindex/build_read.go`.

**Hash do commit / Commit hash:** `4fa481af90` (compiler front end, types2, mirror, noder), `ad30face34` (go command loading + GOPATH root fix)

**Mensagem do commit / Commit message:**
```
feat(cmd/compile): add TypeScript-style imports to the gecko frontend
feat(cmd/go): resolve TypeScript-style imports during package loading
```

**Notes:** `go test cmd/go` TestScript went from 122 intentional failures to 121 (the `.go`-rejection/masked set and `build_static` stay; `cover_list` now passes; nothing regressed). Standard `.go` files keep the Go import grammar.

### Entry: gpm, the gecko package manager

**Título / Title:** `make.bash` now also installs `bin/gpm`, the gecko package manager: it initializes projects and their `gecko.json` manifest, installs dependencies from GitHub under `gpm/modules`, and runs the scripts declared in the manifest.

**Descrição / Description:**
`GOEXPERIMENT=simd ./make.bash` now adds `bin/gpm` to the commands installed in `$GOROOT/bin`. `cmd/go` treats `cmd/gpm` as one of the standard commands whose binaries go into `$GOROOT/bin` (together with `cmd/go` and `cmd/gofmt`); previously every other `cmd/*` main went to `pkg/tool`, so gpm silently never reach `bin/`. The distribution tool listing in `cmd/dist` and the binary manifest in `cmd/distpack` include `cmd/gpm` as well.

`gpm` (Go-curated modules for gecko) is the module-aware package manager for the experimental simd-enabled gecko runtime, implemented under `src/cmd/gpm` per `prompt.md`. It provides:

- `gpm init [<name> | -y]` — scaffold a project: `gecko.json` (name, version, license, type, keywords, scripts, optional `dependencies`/`devDependencies`), `modules.lock` and a starter `main.gk`. `-y` is fully non-interactive; running the new project's `gpm run start` executes `gecko run main.gk`.
- `gpm install <package>[@<version>] [-D]` (`gpm i`) — resolve a dependency (short name or `github.com/owner/repo` origin with optional tag/version), download its `v*` tagged archive into the per-project cache under `gpm/modules` (override the location with `GPM_HOME`), record it in `gecko.json` and `modules.lock`, and resolve its transitive deps.
- `gpm remove <package>` (`gpm rm`), `gpm list`, `gpm search <term>`, `gpm show <package>`, `gpm update [<name>]`, `gpm audit`, `gpm fix`, `gpm run <script> [-- <args>...]`, `gpm version`, `gpm help`.

Internals deployed under `src/cmd/gpm/internal/` (`semver`, `manifest`, `lockfile`, `source`, `storage`, `resolver`, `init`, `ui`) implement npm-style semver ranges with exclusive upper bounds, a GitHub `PackageSource` backend, safe tar.gz extraction with path-traversal rejection, sha256 module checksums, lock-honoring transitive resolution with dedup by repository, and a TUI wizard with a non-TTY fallback. Units tests cover `semver`, `manifest`, `lockfile`, `resolver` and `storage`; the built toolchain was verified end to end (`init -y` → `gecko run main.gk`, install/remove/audit/fix/search/show against real GitHub-hosted modules).

Touched files: `src/cmd/go/internal/load/pkg.go`, `src/cmd/dist/build.go`,
`src/cmd/distpack/pack.go`, `src/cmd/gpm/` (new), `prompt.md`.

**Hash do commit / Commit hash:** `be7a7d7c2f` (cmd/go bin target), `af429bf628` (dist/distpack wiring), `9d540a0695` (implementation), `3e3a8db910` (also handle "gpm" with no arguments)

**Mensagem do commit / Commit message:**
```
feat(cmd/gpm): add the gecko package manager
```

##

**Título / Title:** `make.bash` now installs `bin/gecko` as an alias for `bin/go`, so the gecko toolchain is discoverable under its own command name right after building.

**Descrição / Description:**
Building with `GOEXPERIMENT=simd ./make.bash` installs `bin/go`, `bin/gofmt` and `bin/gecko`. The latter is written by `cmd/dist` right after the toolchain is installed as a real binary (a copy of `bin/go`), so `gecko` is a standalone, relocatable command that can be copied or installed system-wide without following a symlink, and the `$GOROOT/bin` sanity check accepts `gecko` as an expected file. Previously `bin/gecko` had to be created by hand before running `make.bash` (and later was a symlink), and a freshly cleaned checkout only produced `bin/go`.

Use it exactly like `bin/go`:

```
bin/gecko version                # go1.28-devel ... X:simd ...
bin/gecko run examples/09_commit.gk
bin/gecko build -o bin/09_commit examples/09_commit.gk
```

Touched files: `src/cmd/dist/build.go`.

**Hash do commit / Commit hash:** `183d99dfb4` (replaces the `7e75c669b5` symlink approach)

**Mensagem do commit / Commit message:**
```
fix(cmd/dist): write bin/gecko as a real binary instead of a symlink
```

### Entry: gecko null-safe print chains

**Título / Title:** In `.gk` files, a pointer/interface chain used as a `print`/`println`/template argument prints `null` when any pointer in the chain is null, instead of dereferencing it at runtime.

**Descrição / Description:**
Gecko user files (`.gk`) now make pointer/interface selection chains safe to print:

```gecko
var b: *Box = null
println(b.describe())          // null  (instead of a nil dereference)
var r: *Root = null
println(r.outer.inner.text)    // null  (any null link stops the chain)
```

A chain is a base expression followed by one or more field selections, optionally ending in a single method call (`b.describe()`, `r.outer.inner.text`). When the chain is an argument of `print`/`println` or a `${...}` part of a template string (both the `print`/`println` template argument form and templates inside `println`), it is emitted with its base-first pointer/interface guards: once a guard is null, the value prints `null`; otherwise the chain is evaluated and printed normally. Guards only cover receivers that can actually be null (pointers and interfaces); a field selection that is itself nullable is guarded as well so a nil result prints `null`, while a method call at the end of the chain is never re-evaluated for a guard (it may have side effects).

Everything else keeps Go semantics: bare identifiers such as `println(b)` still print the pointer (`0x0`), plain functions and selections on call results are not treated as chains, and `.go` files are unaffected. Templates without chains and `print`/`println` of slices and maps are unchanged.

This also fixes a latent geckoprintln bug: the trailing newline was emitted after `geckoprintend()` reset the print redirect, so it went to stderr and vanished whenever stdout was piped.

Touched files: `src/cmd/compile/internal/noder/{codes,writer,reader}.go`,
`src/runtime/geckoprint.go`, `examples/05_exports.gk`.

**Hash do commit / Commit hash:** `6c0e663508` (feature), `5712b537a3` (runtime newline fix), `6787be01b3` (package-name ICE fix)

**Mensagem do commit / Commit message:**
```
feat(cmd/compile,noder): add gecko null-safe call chains
fix(runtime): emit a trailing newline from geckoprintln while stdout redirect is active
fix(cmd/compile,noder): reject package names as null-safe chain bases
```

### Entry: input-based commit validation example

**Título / Title:** New `examples/09_commit.gk` shows the `input` builtin driving an interactive, pre-commit style validation loop, and fixes `input` dropping piped input past the first line.

**Descrição / Description:**
The new example `examples/09_commit.gk` reads conventional-commit subjects with `input("commit subject> ")` and validates each one against a `CommitValidator` class: the subject must start with a whitelisted type (`feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`), the mandatory `:` and the space after it, and stay within a 72-character limit. Rejected subjects are explained and the prompt repeats; an empty line or EOF ends the session. It exercises `input`, classes with methods, `for`/`while` loops, string indexing and slicing, and `${...}` template strings.

While testing the loop over piped stdin, a real `input` bug surfaced: `geckoinput` read up to 512 bytes at once but discarded every byte after the first `\n`, so with piped input the second and later `input()` calls returned `""`. The read buffer and its read position are now package state, so bytes past the end of the current line carry over to the next call; each `input()` keeps returning the next line until the buffer is exhausted, wrapping around for lines longer than 512 bytes.

Run interactively, or with piped stdin:

```
printf 'fix: add the example\n\n' | go run examples/09_commit.gk
```

Touched files: `src/runtime/geckoinput.go`, `examples/09_commit.gk`.

**Hash do commit / Commit hash:** `fa40b23303` (input fix), `dcae06b083` (example)

**Mensagem do commit / Commit message:**
```
fix(runtime): retain input bytes read past the first line
docs(examples): add an interactive commit validation example (09_commit.gk)
```

### Entry: gecko dynamic literals and recursive print

**Título / Title:** In `.gk` files, `[1, 2, 3]` is a dynamic slice literal and `{k: v}` a dynamic map literal with types inferred from the elements; `print`/`println` render slices and maps (recursively) instead of their raw Go form.

**Descrição / Description:**
Gecko user files (`.gk`) now accept bracketed composite literals without an explicit type:

```gecko
items = [1, 2, 3]            // []int
grid = [[1, 2], [3, 4]]      // [][]int
ages = {"ana": 30, "bia": 25} // map[string]int
```

The literal's type is inferred from its elements: a list is a slice whose element type comes from the element literals (integers → `int`, floats → `float64`, strings → `string`, `true`/`false` → `bool`, `null` → contributes nothing, nested `[...]`/`{...}` recurse, and any non-literal expression, or mixed element types, force that element type to `any`); a key-value literal is a map whose key and value types are inferred the same way (`{true: yes, false: no}` is `map[bool]string`). The Go-style typed form `[]int{...}`/`map[...]...{...}` keeps its explicit type, and an empty dynamic `[]` is not accepted (use `[]T{}`).

In addition, `print`/`println` in `.gk` files print slices and maps with the gecko format instead of the raw Go builtin output (`[3/3]0x...`): any call with at least one slice or map argument and no template string is emitted as a new `exprGeckoPrint` node and lowered to the new runtime helpers `geckoprint`/`geckoprintln`, which box every argument as `interface{}` and print primitives exactly like `println`, slices as `[1 2 3]`, maps as `{k:v, ...}`, arbitrarily nested slice/map structures recursively, and `null` as `null`. This makes the new dynamic arrays/maps examples print their values instead of raw pointer/len dumps.

Standard `.go` files are unaffected (a bracket without a type keeps failing with `missing type in composite literal`, and `println` keeps the Go behavior).

Touched files: `src/cmd/compile/internal/syntax/parser.go`,
`src/cmd/compile/internal/types2/literals.go`,
`src/cmd/compile/internal/noder/{helpers,writer,reader,codes}.go`,
`src/cmd/compile/internal/typecheck/builtin.go`,
`src/runtime/geckoprint.go`.

**Hash do commit / Commit hash:** `93f0d8abeb`

**Mensagem do commit / Commit message:**
```
feat(cmd/compile,syntax,types2,noder,runtime): add gecko dynamic literals and recursive print

In .gk files, a bracketed composite literal without an explicit type is
now a dynamic slice literal whose element type is inferred from the
elements, e.g. [1, 2, 3] is []int and [[1,2],[3,4]] is [][]int. A
key-value literal {k: v, ...} is a map literal inferred the same way;
mixed or non-literal elements degrade to any. The typed Go form
[]T{...} keeps its explicit type.

syntax/parser.go: geckoRbrack parses [ without a following type as a
bare CompositeLit whose ElemList carries the elements; types2/literals.go
infers the composite type (geckoCompositeType/geckoElemType/geckoLitType,
where literals map to their default types, true/false to bool, null to
nil, and compound literals recurse). noder/helpers.go lets untyped
float/complex constants survive idealType.

print and println in .gk print slices and maps recursively instead of
the raw Go form: any call with at least one slice/map argument (and no
template string) is written as an exprGeckoPrint code, read back as a
call to the new runtime geckoprint/geckoprintln helpers, which box every
argument as interface{} and print primitives like println, slices as
[1 2 3], maps as {k:v, ...}, nested structures recursively, and null as
null.
```

### Entry: numbered examples and arrays/maps examples

**Título / Title:** The `.gk` examples are renumbered into per-feature files (`00_hello.gk`...`08_maps.gk`) and new arrays and maps examples cover the dynamic literal and recursive print features.

**Descrição / Description:**
The `examples/` directory now follows a numbered, per-feature layout so the examples build up feature by feature:

- `00_hello.gk`, `01_conditions.gk` (if/switch), `02_loops.gk` (for/while/range), `03_templates.gk`, `04_classes.gk`, `05_exports.gk` (+ `exportlib/lib.gk`) renumber the previous `hello.gk`, `if.gk`, `switch.gk`, `for.gk`, `while.gk`, `templates.gk`, `classes.gk` and `export.gk`.
- `06_null.gk` — the gecko `null` literal in comparisons, typed assignments and pointer arguments (a class pointer field printed via the recursive printer shows `null`).
- `07_arrays.gk` — dynamic and typed slice literals, `append` (including the variadic `[2, 3]...` form), slicing `[1:3]`/`[:2]`, `len`, index read/write, multi-dimensional slices (`grid[1][1]`), `[]string`/`[]float64`/`[]any` slices, `range`, `make`/`copy`, and slices as function arguments/results.
- `08_maps.gk` — dynamic and typed map literals, `int`/`bool` keys, maps of slices and nested maps, heterogeneous `map[string]any` values, empty and `null` maps (printed as `{}`), `len`, get/set, `delete`, iteration, and maps as function arguments/results.

All run with the simd-enabled `bin/go` (e.g. `go run examples/08_maps.gk` from `src/`; `05_exports.gk` must be run from `examples/`).

Touched files: `examples/00_hello.gk`,
`examples/01_conditions.gk`,
`examples/02_loops.gk`,
`examples/03_templates.gk`,
`examples/04_classes.gk`,
`examples/05_exports.gk`,
`examples/06_null.gk`,
`examples/07_arrays.gk`,
`examples/08_maps.gk`,
`examples/exportlib/lib.gk`.

**Hash do commit / Commit hash:** `a574af5b6d`

**Mensagem do commit / Commit message:**
```
docs(examples): number the per-feature example files and add arrays/maps

Rename the per-feature examples (hello, if, for, while, switch,
templates, classes, export -> 00_hello, 01_conditions, 02_loops,
03_templates, 04_classes, 05_exports) and add:

- 06_null.gk: the gecko null literal in comparisons, typed assignments
  and pointer args
- 07_arrays.gk: dynamic and typed slice literals, append (including the
  variadic form), slicing, len, index read/write, multi-dimensional
  slices, range, make/copy, and slices as arguments and results
- 08_maps.gk: dynamic and typed map literals, int/bool keys, maps of
  slices and nested maps, heterogeneous map[string]any values, empty and
  null maps, len, get/set, delete, iteration, and maps as arguments and
  results

Update the exportlib library example header to point at 05_exports.gk.
```

### Entry: gecko export example and qualified new

**Título / Title:** `new pkg.Class(args)` is supported in `.gk` files, and the export keyword now has a worked example (`examples/export.gk` + `examples/exportlib`).

**Descrição / Description:**
The `new` expression now accepts a qualified class name, so a class defined in another gecko package can be constructed directly: `p = new exportlib.Person("Bia")`. Previously only `new Class(...)` with a bare name parsed (`NewExpr.Type` was already a general `Expr`, so this is just a parser change to use `qualifiedName`).

The new example demonstrates the export keyword end to end:
- `examples/exportlib/lib.gk` is a library gecko package that exports a lowercase function (`greet`), a var (`greeting`), a const (`version`), a shorthand export (`export upper`), a list-form export (`export {myMax, version}`) and a class (`Person`) whose field (`this.name`) and methods (`greet`, `Loud`) are reachable across packages. It also contains names with capital letters that stay private (`PrivateGreeting`, `SecretGreeting`, `Secret`).
- `examples/export.gk` (package main) imports the library and uses every exported name; the commented-out references show the three private names fail to compile with `undefined: exportlib.<name>`.

Run it with `go run ./export.gk` (from `examples/`, using a simd-enabled `bin/go`).

Touched files: `src/cmd/compile/internal/syntax/parser.go`,
`examples/export.gk`,
`examples/exportlib/lib.gk`.

**Hash do commit / Commit hash:** `172d5d8f0e`

**Mensagem do commit / Commit message:**
```
feat(syntax,examples): support qualified new expressions and add the export example

Allow new pkg.Class(args) in gecko source by parsing the class with
qualifiedName instead of a bare name (NewExpr.Type is already an Expr).
This makes cross-package class use in examples natural.

Add examples/export.gk and examples/exportlib/lib.gk, which demonstrate
the gecko export keyword end to end: exported lowercase functions,
variables and constants, the shorthand and list export forms, a class
whose fields and methods are reachable across packages, and (commented
out) the private names that do not compile despite their capital
letters.
```

### Entry: gecko export keyword

**Título / Title:** In `.gk` files, the `export` keyword controls visibility: declarations without it are private even if their name starts with an uppercase letter, and `export` makes names visible to importers regardless of capitalization.

**Descrição / Description:**
Gecko user files (`.gk`) replace Go's "first letter uppercase" export rule with an explicit `export` keyword:

```gecko
export func up(): string { "up" }   // exported, lowercase
func Hidden(): string { ... }       // private despite the capital H
export var value = 42               // exported var
export { printer, Person }          // export list form
export number, Logger               // shorthand form (before the declaration)
```

Every top-level declaration without `export` is private, even if its name starts with an uppercase letter. `export` may appear before `func`, `var`, `const` and `class` (as well as the shorthand `export name` and the list form `export {a, b}`, which apply to declarations with those names, including ones declared later in the file or in another file of the package). Fields and methods of gecko classes are visible to importing packages regardless of capitalization, so `p.name` and `p.greet()` work across package boundaries; the class itself must still be exported to be reachable. Referencing a name that does not exist in any file of the package is a compile error (`cannot export undefined name ...`).

The visibility override is carried in the unified IR export data as two new pieces of information: a per-object override bit stored at the start of each object element and a per-package gecko flag stored right after the package name (implemented as `pkgbits.V5` and the `GeckoExport` field). The public-API filter in the exporter uses the override instead of the capitalization rule, and every unified IR reader (the compiler linker, `cmd/compile/internal/importer`, `cmd/compile/internal/testimporter`, `go/internal/gcimporter` for `go/types`, and the vendored x/tools gcimporter) was updated to consume the new bits; `go/types` is regenerated from `types2` and keeps the gecko-specific methods unexported so no new public API is introduced. Standard `.go` files keep Go's capitalization rule.

Touched files: `src/cmd/compile/internal/syntax/{tokens,scanner,parser,nodes,printer,walk,token_string}.go`,
`src/cmd/compile/internal/types2/{object,package,check,resolver,scope,named}.go`,
`src/cmd/compile/internal/types2/testdata/local/gecko_export.gk`,
`src/cmd/compile/internal/noder/{writer,reader,unified}.go`,
`src/cmd/compile/internal/{importer,testimporter}/ureader.go`,
`src/internal/pkgbits/version.go`,
`src/go/internal/gcimporter/ureader.go`,
`src/go/types/{object,package,scope,named,generate_test,sizeof_test}.go`,
`src/cmd/vendor/golang.org/x/tools/internal/{gcimporter/ureader.go,pkgbits/version.go}`.

**Hash do commit / Commit hash:** `2901cc85eb`

**Mensagem do commit / Commit message:**
```
feat(cmd/compile,syntax,types2,noder,gcimporter): add the gecko export keyword

In .gk files, the export keyword marks a package-level declaration as
visible to importing packages regardless of capitalization; declarations
without it are private even when they start with an uppercase letter.
Fields and methods of gecko classes are likewise visible to importers
regardless of capitalization.

The syntax package parses export before declaration keywords, the
shorthand form 'export name', and the list form 'export {a, b}'. types2
records an "exported" override per object (SetGeckoExported) and marks
the package as gecko (Package.MarkGecko), and object.Exported consults
the override and the per-package gecko flag for fields and methods. The
compiler's unified IR writer emits the override bit and the package
flag (pkgbits V5, GeckoExport field) at the start of each object entry
and after the package name; all unified IR readers (noder/reader,
cmd/compile/internal/importer, cmd/compile/internal/testimporter,
go/internal/gcimporter, and the vendored x/tools gcimporter) consume or
discard the extra bits, and the go/types mirror is regenerated
(gecko-specific methods stay unexported there to avoid public API
changes). The writeUnifiedExport public-root filter uses the override
when deciding whether an object is part of a package's public API.

Standard .go files keep Go's capitalization rule.
```

### Entry: gecko template strings

**Título / Title:** In `.gk` files, backtick string literals interpolate `\${...}` expressions as `print`/`println` arguments.

**Descrição / Description:**
Gecko user files (`.gk`) treat a raw (backtick) string containing `\${...}` as a template string when it is used directly as an argument of `print` or `println`:

```gecko
name = "Ana"
println(`hi ${name}, ${2 + 5} squared is ${(2 + 5) * (2 + 5)}`)
```

Each `${...}` interpolates its expression into the output, formatted exactly like a `print` argument (so ints, floats, bools, strings, slices, pointers, interfaces, etc. print the same way they do in a plain `print` call). `print` emits the interpolated text with no separators; `println` separates multiple arguments with a space and appends a trailing newline, matching the builtins' behavior. The expression inside `${...}` may use any gecko/Go expression, including nested calls, struct/map/slice literals and sub-templates are not re-interpolated.

The template is implemented entirely in the compiler: the syntax parser turns the backtick literal into a `TemplateLit` node and parses each interpolated sub-expression with a sub-parser mapped back onto the template's source positions; `types2` type-checks the sub-expressions (the whole template types as `string`); and the noder flattens the literal segments and sub-expressions into a single `print` call (`exprPrintTemplate`), appending a trailing `"\n"` for `println`. Using a template anywhere other than as a direct `print`/`println` argument is a compile error. Standard `.go` files keep Go's raw strings (a backtick literal is a plain string there; `\${` stays literal text).

Touched files: `src/cmd/compile/internal/syntax/nodes.go`,
`src/cmd/compile/internal/syntax/parser.go`,
`src/cmd/compile/internal/syntax/printer.go`,
`src/cmd/compile/internal/syntax/walk.go`,
`src/cmd/compile/internal/types2/expr.go`,
`src/cmd/compile/internal/noder/{codes.go,writer.go,reader.go}`,
`examples/templates.gk`.

**Hash do commit / Commit hash:** `07db28adfe`

**Mensagem do commit / Commit message:**
```
feat(cmd/compile,syntax,types2,noder): add gecko template strings

In .gk files, raw (backtick) string literals containing ${...} define
gecko template strings, e.g. println(`sua soma é: ${2 + 5}`). A
template is only meaningful as a direct argument of print and println;
the front end flattens the literal segments and interpolated
sub-expressions into a single print call, appending a trailing newline
for println, so interpolated values are formatted exactly like print's
other arguments.

The syntax package treats a backtick literal containing ${ as a new
TemplateLit node whose parts are parsed with a sub-parser over each
interpolated expression; types2 validates the sub-expressions (the
template itself types as string); noder emits a new exprPrintTemplate
code read back as an OPRINT call. Standard .go files are unaffected.

Example: examples/templates.gk.
```

### Entry: gecko `null` replaces `nil`

**Título / Title:** In `.gk` files the nil literal is spelled `null`.

**Descrição / Description:**
Gecko user files (`.gk`) spell the nil value as `null`, matching the
conventional name used by most C-like languages. In the type checker,
an unresolved `null` identifier in a `.gk` file resolves to the
predeclared nil object and behaves exactly like Go's `nil`: it can be
compared against any nilable value (`m == null`), assigned to any
pointer/slice/map/interface/function/channel type
(`var p: *int = null`) and passed as an argument. Writing `nil` in a
`.gk` file is a type-checking error with the migration hint
`nil is not allowed in gecko; use null instead`.

The change is mirrored in `go/types` (both `ident` implementations
resolve `null` and reject `nil` under the gecko file rule); `go/parser`
and `go/printer` need no change because `null` already parses and
prints as an ordinary identifier. In `.go` files nothing changes:
`nil` keeps its usual meaning and `null` stays an ordinary (possibly
undeclared) identifier.

Touched files: `src/cmd/compile/internal/types2/typexpr.go`,
`src/go/types/typexpr.go`.

**Hash do commit / Commit hash:** `6e7586713d`

**Mensagem do commit / Commit message:**
```
feat(types2,go/types): rename the nil literal to null in gecko

In .gk files, null is the name of the nil value: an unresolved null
identifier resolves to the predeclared nil object and behaves exactly
like nil (comparisons, typed nil assignments, arguments). Writing nil
in a .gk file is a type-checking error with the hint
'use null instead'. In .go files nothing changes: nil keeps its usual
meaning and null stays an ordinary (possibly undeclared) identifier.
```

### Entry: gecko classes and `new` expressions

**Título / Title:** `class` declarations and `new Person(args)` expressions in `.gk`.

**Descrição / Description:**
Gecko user files (`.gk`) gain class declarations as the primary way to
define types:

```gecko
class Person() {
    constructor(n: string, a: int) {
        this.name = n
        this.age = a
    }
    Greet(): string {
        return "hi " + this.name
    }
}

p = new Person("Ana", 30)
```

A `class Name()` declaration has an explicit empty parameter list, a
body of methods, and desugars in the front end into: (1) a type
declaration whose underlying type is a struct with one field per
`this.<name>` reference in the class methods, and (2) one `FuncDecl`
per method with an implicit receiver named `this` of type `*Name`.
Fields are created implicitly by `this.<name> = value` assignments and
their types are inferred from the constructor: a literal RHS fixes the
type (`int`, `float64`, `rune`, `complex128`, `string`), a constructor
parameter with an explicit type propagates it, and everything else
(untyped constructor parameters, assignments in other methods, reads
without prior assignment, and constructor parameters declared without a
`:` type) becomes `any`. Method calls on `this` are not treated as
field references.

`new Name(args...)` allocates a `*Name` and, when the class has a
`constructor` method, calls it with the arguments converted to the
constructor's parameter types; the constructor's result value is
discarded. A class without a constructor can still be allocated; extra
arguments are rejected. Constructor parameters without an explicit type
default to `any`.

`class` and `new` are keywords only for `.gk` files; in `.go` they stay
ordinary identifiers and the `new(T)` builtin is unaffected, as are
`type`, `struct` and all other Go type machinery (such constructs are
rejected in `.gk` with a hint to define a class instead). A method
declares results with the gecko colon syntax (`greet(): string`) or
none at all; there is no `void` result type.

Touched files: `src/cmd/compile/internal/syntax/{parser,nodes,printer,
scanner,tokens,token_string,walk}.go`,
`src/cmd/compile/internal/types2/{class,check,expr,typexpr,signature}.
go`, `src/cmd/compile/internal/noder/{codes,writer,reader}.go`,
`examples/classes.gk`.

**Hash do commit / Commit hash:** `d5014e8c24`

**Mensagem do commit / Commit message:**
```
feat(cmd/compile,syntax,types2,noder): add gecko class declarations and new expressions

For .gk files, 'class Name()' is a new top-level declaration that
desugars into a type declaration (underlying type: a struct whose
fields are inferred from this.<field> references in the class methods)
plus one FuncDecl per method with an implicit 'this' receiver of type
*Name. Constructor parameters without an explicit type default to any.

A 'new Name(args)' expression desugars at the noder level into an
allocation (ONEW) followed by a call to the constructor method, with
arguments converted to the constructor's parameter types. Construction
without a constructor is supported; extra arguments are rejected.

'class' and 'new' are keywords only for .gk files; in .go they remain
ordinary identifiers and the new(T) builtin is unaffected.
```

### Entry: user-facing .gk source extension

**Título / Title:** Support the `.gk` extension for user source files.

**Descrição / Description:**
Gecko expects user sources to use the `.gk` extension (plain Go syntax)
while the standard library stays written in `.go`. `go/build` now accepts
both extensions, and the `go` command resolves `.gk` packages in file mode
(`go run file.gk`) and directory mode (`go run .`) by extending
`fsys.IsGoDir`, `modindex` and `imports.ScanDir` to recognize `.gk`.
Explicitly naming a user `.go` file on the command line is rejected with
`gecko does not compile .go files; use the .gk extension`; `.go` files
inside the GOROOT tree (stdlib `src/` and `test/`) are still allowed so
that `test/simd` and the simd testdata subprocess still work. The `go run`
executable name is derived by stripping the `.gk` suffix. Adds
`examples/hello.gk` and `examples/go.mod` and documents the fork layout in
`AGENTS.md`.

Touched files: `src/go/build/build.go`,
`src/cmd/go/internal/{fsys,imports,load,modcmd,modindex,modload,run}`.

**Hash do commit / Commit hash:** `a4677a7565606c036c3ae775f4505b078c8ea544`

**Mensagem do commit / Commit message:**
```
feat(cmd/go): support the .gk extension for user source files

Gecko expects user sources to use the .gk extension while the standard
library stays written in .go. go/build now accepts both extensions via
the new isGoFileName/isGoTestFileName helpers, and the go command
resolves .gk packages both in file mode (go run file.gk) and directory
mode (go run .) by extending fsys.IsGoDir, modindex and imports.ScanDir
to recognize .gk.

Explicitly naming a user .go file on the command line is rejected with
'gecko does not compile .go files; use the .gk extension'; .go files
inside the GOROOT tree (stdlib and test/) are still allowed so that
test/simd and the simd testdata subprocess keep working. The go run
executable name is derived by stripping the .gk suffix.

Adds examples/hello.gk and AGENTS.md documenting the fork layout.
```

### Entry: native print/println/input builtins

**Título / Title:** Add native `print`, `println`, and `input` builtins with stdout/stdin I/O.

**Descrição / Description:**
Gecko user code can now use the builtins `print` (stdout, no newline),
`println` (stdout, newline) and `input(prompt)` (writes the prompt to
stdout, reads one line from stdin, returns it as a `string`; EOF yields
the empty string) without importing `fmt`. `print`/`println` are routed
to stdout instead of stderr: `walkPrint` now emits
`printlock; geckoprintbegin <printX/nl>* geckoprintend; printunlock`,
and the new runtime hooks `geckoprintbegin`/`geckoprintend` toggle the
`g.writefd` field so that `gwrite` writes to file descriptor 1 while
it is set. Runtime-internal diagnostics (e.g. `panic` messages) still go
to stderr because they do not set `writefd`. The `input` builtin is a new
compiler op `OINPUT` (placed at the end of the `ir.Op` enum), typechecked
by `tcInput`, lowered by `walkInput` into a call to the new runtime
function `geckoinput(prompt string) string` (runtime/geckoinput.go), and
exposed to type checking through the new predeclared identifier in the
types2/go/types universes. The `OINPUT` op is wired through escape
analysis (argument discarding), inlining flatness heuristics, the walker
and the unified-IR reader/writer via the generic builtin path, matching
how `print`/`println` are handled; no noder changes were required.

Touched files: `src/runtime/{runtime2,print,geckoinput}.go`,
`src/cmd/compile/internal/ir/{node,expr,fmt,op_string}.go`,
`src/cmd/compile/internal/types2/{universe,builtins}.go`,
`src/cmd/compile/internal/typecheck/{universe,func,typecheck,const,stmt}.go`
and `_builtin/runtime.go`,
`src/cmd/compile/internal/walk/{builtin,expr,order,stmt}.go`,
`src/cmd/compile/internal/escape/{call,expr}.go`,
`src/cmd/compile/internal/inline/inlheur/analyze_func_flags.go`,
`src/go/types/{universe,builtins,builtins_test}.go`,
`test/geckoio.go|out`, `examples/hello.gk`.

**Hash do commit / Commit hash:** `c928d6244c28a672a7ed4e6c184c61b90433815f`

**Mensagem do commit / Commit message:**
```
feat(runtime,cmd/compile): add print/println/input builtins with stdout I/O

User gecko code can now print to stdout with the print and println
builtins, and read one line from stdin with the new input(prompt)
builtin, without importing fmt. print/println route through the runtime
diagnostic printer (formatting semantics are unchanged) but land on file
descriptor 1 instead of stderr: walkPrint wraps the print args in
geckoprintbegin/geckoprintend, which set and clear g.writefd, and gwrite
writes to fd 1 while the flag is set. Runtime-internal diagnostics keep
going to stderr.

input is a predeclared builtin backed by the new OINPUT op and the
runtime geckoinput function: it writes its string argument to stdout,
reads a line (stripping CR) from stdin, and returns it; EOF returns "".
OINPUT is threaded through typechecking, escape analysis, the walker and
the unified IR exactly like print/println, and the types2/go/types
universes both declare it.
```

### Notes

- Testing tips for this feature:
  - `printf 'Alice\n' | bin/go run examples/hello.gk` (from `src/`).
  - `printf '' | bin/go run test/geckoio.gk` must print `PQ\nNgot  len 0\n`.
  - `bin/go test cmd/internal/testdir -run 'Test/geckoio.go'`.
  - `bin/go test go/types cmd/compile/internal/types2`.
- `input` with a non-string argument or wrong arity is a compile-time
  error (`argument to input must be a string, have int` / invalid
  operation).

### Entry: gecko control-flow syntax

**Título / Title:** Mandatory clause parentheses and the `while` keyword for `.gk` source files.

**Descrição / Description:**
Gecko user files (`.gk`) now require parentheses around every
`if`/`for`/`switch` clause — `if (x)`, `for (init; cond; post)`,
`for (i := range items)`, `for (;;)`. `switch (v := x.(type))` keeps
working because `(type)` is only legal inside the parenthesized guard.
A bare clause is rejected at parse time with
`missing ( after if (mandatory in gecko)`. The new `while (cond) body`
loop is provided as sugar for `for (cond) body` (no init/post); an empty
condition `while ()` is an infinite loop like `for (;;)`, and `break`/
`continue` work as usual. Standard library files (`.go`) are untouched:
`.go` keeps the full Go grammar, and `while` remains a plain identifier
there, so the behavior is gated purely on the source file extension.

The change spans both the compiler front end and the gofmt/vet surface so
they always agree. In `cmd/compile/internal/syntax` the `header` parser
generalizes its clause terminator (allowing both `{` and `)`) and the
scanner recognizes `_While` only when `gk` is set (derived from the
`.gk` filename suffix); a `while` statement parses straight into a
`*syntax.ForStmt`, so no noder/IR changes were needed. On the Go side
`go/token` gains a `WHILE` token inside the keyword range (the `go`
scanner maps `while` to `WHILE` only for `.gk`, so `.go` identifiers
are unaffected), `go/parser` enforces the parentheses and produces a new
`ast.WhileStmt`, `go/printer` gains a `GeckoParens` mode
(`cmd/gofmt` enables it for `.gk`) that prints `if/for/switch` clauses
(including type switches and range clauses) parenthesized, and
`go/types` type-checks `WhileStmt`.

Touched files: `src/cmd/compile/internal/syntax/{parser,scanner,tokens,
token_string}.go`, `src/go/token/token.go`, `src/go/scanner/scanner.go`,
`src/go/parser/parser.go|parser_test.go`, `src/go/ast/{ast,walk}.go`,
`src/go/printer/{printer,nodes,printer_test}.go`, `src/go/types/stmt.go`,
`src/cmd/gofmt/gofmt.go`, `examples/{hello,if,for,while,switch}.gk`,
`src/go/printer/testdata/gecko.gk`.

**Hash do commit / Commit hash:** `cd1a93f95adaa816b27280f740449fcb6a36cca6`

**Mensagem do commit / Commit message:**
```
feat(cmd/compile,syntax,go): require parenthesized if/for/switch clauses and add while for .gk

Gecko user files (.gk) now require parentheses around every
if/for/switch clause: if (x), for (init; cond; post),
for (i := range items), for (;;), switch (v := x.(type)). A bare
clause is rejected at parse time with "missing ( after ... (mandatory
in gecko)". A new while (cond) body loop is provided as sugar for
for (cond) body; while () is an infinite loop like for (;;).

In cmd/compile/internal/syntax the header parser generalizes its
clause terminator to accept either '{' or ')' and the scanner
recognizes the new _While keyword, both gated on a gk flag derived
from the .gk filename suffix. A while statement parses directly into
a *syntax.ForStmt (no init/post), so the noder and IR are untouched.
Standard library .go files keep the full Go grammar; while remains a
plain identifier there.

On the Go tooling side go/token gains a WHILE token inside the
keyword range, go/scanner maps "while" to WHILE only for .gk sources,
go/parser enforces the parentheses and emits a new ast.WhileStmt,
go/printer adds a GeckoParens mode that prints if/for/switch clauses
(including type switches and range clauses) parenthesized and renders
WhileStmt, cmd/gofmt enables that mode for .gk, and go/types
type-checks WhileStmt.

Splits the examples into per-feature files (if.gk, for.gk, while.gk,
switch.gk) and adds parser/printer regression coverage
(go/parser TestGecko*, go/printer gecko testdata).
```

### Entry: gecko declare-or-assign `=` and mandatory `:` type annotations

**Título / Title:** Gecko `x = e` declare-or-assign replaces `:=`, and explicit types require `:`.

**Descrição / Description:**
Gecko user files (`.gk`) drop the `:=` short variable declaration in
favor of a Python-like declare-or-assign rule: `x = e` assigns to `x`
when the name is already visible anywhere in the lexical environment and
declares a fresh variable in the current block otherwise. This also
applies to `for (i = 0; ...)` init clauses and to range iteration
variables (`for (i = range items)`), and to the parentheses-less type
switch guard (`switch (t = v.(type))`). Writing `:=` in a `.gk` file is
a parse error with the migration hint
`':=' is not allowed in gecko; use '=' instead`. Unparenthesized gecko
var declarations are ignored by the declare-or-assign rule, so
`x = (y) = ...` etc. remain Go-consistent; multi-name forms
(`i, v = range items`) declare each missing name.

In addition, an explicit type in `.gk` must be separated from its name
with a colon: `var typedWord: string = "Hello World"`,
`func Logger(level: string, message: string)`,
`func Login(name: string, password: string): bool`,
`var a, b: int = 1, 2`, and `const N: int = 5`. A missing colon is a
parse error (`missing ':' between the variable name and its type
(gecko)` for var/const, `... between the parameter name and its type
...` for params, and `missing ':' before the function result type
(gecko)` for results). Struct fields and type parameter lists keep the
Go syntax (no colon); unnamed (type-only) parameters need no colon.

The change is mirrored in the compiler front end and the go tooling so
they always agree. In `cmd/compile/internal/types2`, `shortVarDecl` was
refactored into `declareVars(pos, lhs, rhs, chain)`; the new
`geckoVarDecl` runs with `chain = true`, resolving each lhs through the
full environment (`check.lookup`) instead of only the innermost block.
The `syntax` parser allows `=` (and errors on `:=`) in the assign and
simpleStmt paths, requires `:` in `varDecl`/`constDecl`/`paramDeclOrNil`
and before `funcResult`, and accepts `switch (t = v.(type))` guards.
The noder's `forStmt` range helper consults `info.Defs` so
gecko-declared range variables code-generate correctly.

On the Go tooling side, `go/parser` mirrors the `:=` error, the `:`
annotations (via the new `atGeckoColon` helper and `parseResults`), and
the `=` type-switch guard; `go/types` mirrors the declare-or-assign
logic (with `gecko.go` helpers `geckoFile`/`geckoNewVar`, hand-maintained
because the generated files must not reference `syntax.Pos`), and
`go/printer` gains a `GeckoColons` mode (`cmd/gofmt` enables it for
`.gk`) that prints `var x: int`, `func f(a: int): int`, and
`a, b: int = 1, 2` colons. The `go/types` generated files are kept in
sync by the existing `-run=TestGenerate -write=all` mechanism.

Touched files: `src/cmd/compile/internal/syntax/parser.go`,
`src/cmd/compile/internal/types2/{assignments,range,stmt}.go`,
`src/cmd/compile/internal/types2/gecko.go`,
`src/cmd/compile/internal/noder/writer.go`,
`src/go/parser/parser.go|parser_test.go`,
`src/go/types/{assignments,range,stmt,check_test}.go`,
`src/go/types/gecko.go`,
`src/go/printer/{printer,nodes,printer_test}.go`,
`src/cmd/gofmt/gofmt.go`, `src/go/printer/testdata/gecko.gk`,
`examples/{hello,if,for,while,switch}.gk`.

**Hash do commit / Commit hash:** `7189aa8a5ecff5fb29b95674a88bcec2e8d6c3d5`

**Mensagem do commit / Commit message:**
```
feat(cmd/compile,go): replace := with the gecko = declare-or-assign and require : on types

Gecko user files (.gk) replace the := short variable declaration with
a Python-like declare-or-assign rule: x = e assigns to x when the name
is visible anywhere in the lexical environment and declares a fresh
variable in the current block otherwise, including in for init clauses
(i = 0), range iteration variables (for (i = range items)) and type
switch guards (switch (t = v.(type))). A := in a .gk file is a parse
error: ":=' is not allowed in gecko; use '=' instead".

Explicit types require a colon in .gk: var x: int = 5, func f(a: int),
func f(): bool, const N: int, var a, b: int = 1, 2. Missing colons are
parse errors. Struct fields and type parameter lists keep the Go
syntax; unnamed (type-only) parameters do not need a colon.

In cmd/compile/internal/types2, shortVarDecl is refactored into
declareVars(pos, lhs, rhs, chain); the new geckoVarDecl runs with
chain=true and resolves each lhs through the full environment
(check.lookup) instead of only the innermost block. The syntax parser
errors on :=, requires ':' in var/const/param/result type annotations,
and accepts '=' in type switch guards. The noder forStmt range helper
consults Defs so gecko-declared range variables code-generate.

On the Go tooling side, go/parser and go/types mirror the := error,
the ':' annotations and the declare-or-assign semantics (go/types
gecko helpers live in a hand-maintained go/types/gecko.go mirroring
cmd/compile/internal/types2/gecko.go; generated files stay in sync via
TestGenerate). go/printer gains a GeckoColons mode that prints
var/param/result colons; cmd/gofmt enables it for .gk. Adds parser
(TestGeckoColonEqualsError) and typechecker
(TestGeckoDeclareOrAssign) regression tests and migrates the examples
and the go/printer gecko testdata to the new syntax.
```

### Notes

- Testing tips for this feature:
  - `bin/go run ../examples/hello.gk` (from `src/`) exercises the typed
    `var typedWord: string` annotation and the `=` declare-or-assign.
  - `bin/go run ../examples/for.gk` exercises `for (i = 0; ...)`,
    `for (i = range items)` and `for (i, v = range items)`.
  - `bin/gofmt -d ../examples/for.gk` shows no diff; gofmt keeps the
    colons (`GeckoColons`) on `.gk`.
  - `bin/go test cmd/compile/internal/syntax cmd/compile/internal/types2
    cmd/compile/internal/noder go/parser go/types go/printer cmd/gofmt`.
- `while` stays a plain identifier in `.go` files (stdlib unaffected);
  a `.gk` file using `while` as a variable name is a syntax error.
- Parens must wrap the whole clause: `switch (y := x; y)`,
  `switch (t := v.(type))` are the `.gk` spellings; bare `switch x` is
  rejected.

### Notes

- The module index is cached in the build cache keyed by content hash and
  mtime; after changing `.gk` handling rules, run `bin/go clean -cache`
  before re-testing directory mode.
- Suggested sanity checks: `bin/go run ../examples/hello.gk` (from `src/`),
  `bin/go run .` in a flat `.gk` module outside GOROOT,
  `bin/go run /tmp/foo.go` (must be rejected, rc=1), `bin/go test ./simd`,
  `bin/go test cmd/internal/testdir -run 'Test/simd'`, `bin/go build std`.