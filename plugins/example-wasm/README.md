# Example Butter WASM Plugin

Demonstrates the Butter WASM plugin ABI. Implements `pre_http` to tag
each request with metadata visible to subsequent hooks and logging.

## Building

Requires [TinyGo](https://tinygo.org/getting-started/install/) ≥ 0.34.

```bash
# Via justfile (recommended)
just build-example-wasm

# Manually
tinygo build -o example-wasm.wasm -scheduler=none -target=wasi ./plugins/example-wasm/
```

The output `example-wasm.wasm` is placed in `plugins/example-wasm/`.

## Configuration

```yaml
# config.yaml
wasm_plugins:
  - name: example-wasm
    path: ./plugins/example-wasm/example-wasm.wasm
    config:
      tag: my-deployment   # accessible via pdk.GetConfig("tag") inside the plugin
```

## Writing Your Own Plugin

1. Copy this directory as a starting point.
2. Import the Extism Go PDK:
   ```
   go get github.com/extism/go-pdk
   ```
3. Export any subset of: `pre_http`, `post_http`, `pre_llm`, `post_llm`.
4. Read the incoming [sdk.Request] from `pdk.Input()` and write a
   [sdk.Response] via `pdk.Output()`. Both are JSON-encoded.

See [`plugin/sdk/sdk.go`](../../plugin/sdk/sdk.go) for the full type
documentation.

## ABI Reference

| Hook | Called when | Can short-circuit |
|------|-------------|:-----------------:|
| `pre_http` | Before routing, before provider selection | ✅ |
| `post_http` | After provider response received | — |
| `pre_llm` | Immediately before provider call | ✅ |
| `post_llm` | After provider call, before cache store | — |

All hooks are optional. Unexported hooks are silently skipped.
