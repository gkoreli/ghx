## 1

Question:
Where is middleware composition implemented?

Answer:
Middleware composition is implemented primarily in [`src/compose.ts`](/Users/goga/Documents/goga/ghx/src/compose.ts), and it is invoked by the request pipeline in [`src/hono-base.ts`](/Users/goga/Documents/goga/ghx/src/hono-base.ts). The higher-level combine helpers in [`src/middleware/combine/index.ts`](/Users/goga/Documents/goga/ghx/src/middleware/combine/index.ts) build on the same `compose` helper. Confidence: high.

Relevant files:
- [`src/compose.ts`](/Users/goga/Documents/goga/ghx/src/compose.ts): core middleware dispatcher; sequentially invokes handlers, manages `next()`, error handling, and not-found handling.
- [`src/hono-base.ts`](/Users/goga/Documents/goga/ghx/src/hono-base.ts): request dispatch path calls `compose(matchResult[0], this.errorHandler, this.#notFoundHandler)`.
- [`src/middleware/combine/index.ts`](/Users/goga/Documents/goga/ghx/src/middleware/combine/index.ts): middleware combinators; `every` explicitly composes handlers through `compose`.

Evidence:
- `src/compose.ts` defines `export const compose = ...` and its `dispatch(i)` loop calls each middleware in order, enforces single `next()` progression, and applies `onError`/`onNotFound`.
- `src/hono-base.ts` at the routing/dispatch site uses `const composed = compose(matchResult[0], this.errorHandler, this.#notFoundHandler)` and then awaits `composed(c)`.
- `src/hono-base.ts` `this.use(...)` only registers handlers with `METHOD_NAME_ALL`; it does not itself perform composition.
- `src/middleware/combine/index.ts` `every(...)` wraps its handlers into `compose(...)`, confirming the shared composition primitive.

Commands run:
- `ghx explore honojs/hono`
- `ghx tree honojs/hono src --depth 2`
- `ghx read honojs/hono src/compose.ts src/hono-base.ts src/hono.ts src/middleware/combine/* --map`
- `ghx read --help`
- `ghx tree honojs/hono src/middleware/combine --depth 2`
- `ghx read honojs/hono src/compose.ts src/hono-base.ts src/hono.ts src/middleware/combine/index.ts --map`
- `ghx read honojs/hono src/compose.ts src/hono-base.ts src/middleware/combine/index.ts --grep "compose"`
- `ghx read honojs/hono src/compose.ts --lines 1-120 src/hono-base.ts --lines 400-460 src/middleware/combine/index.ts --lines 1-160`
- `ghx read honojs/hono src/hono-base.ts --lines 150-260`

Backends used:
- ghx remote tree/map/grep/read

Uncertainty:
- I did not inspect every middleware package for custom composition logic. The core shared implementation is clearly in `src/compose.ts`, but some individual middleware modules may have their own small control-flow wrappers.

Suggested next reads:
- `ghx read honojs/hono src/compose.ts --lines 1-120`
- `ghx read honojs/hono src/hono-base.ts --lines 400-460`


----
## 2

Question:
How do errors propagate through the middleware composition path in `honojs/hono`?

Answer:
Errors thrown by a middleware handler are caught in `src/compose.ts`, converted into the active `onError` path when the thrown value is an `Error`, and then surfaced back to `HonoBase` as either a `Response` or a rethrown non-`Error`. `HonoBase` wires this together by passing its current `errorHandler` and `notFoundHandler` into `compose(...)`, and its private `#handleError` delegates final error-to-response conversion to `this.errorHandler(err, c)`. Confidence: high.

Relevant files:
- [`src/compose.ts`](/Users/goga/Documents/goga/ghx/src/compose.ts): core error-catching and `onError`/`onNotFound` handling inside the middleware dispatcher.
- [`src/hono-base.ts`](/Users/goga/Documents/goga/ghx/src/hono-base.ts): request dispatch path, default/custom error handler wiring, and final error-to-response conversion.
- [`src/middleware/combine/index.ts`](/Users/goga/Documents/goga/ghx/src/middleware/combine/index.ts): higher-level combinators; `every()` composes nested middleware through the same primitive.

Evidence:
- In [`src/compose.ts`](/Users/goga/Documents/goga/ghx/src/compose.ts), the dispatcher wraps `handler(context, () => dispatch(i + 1))` in `try/catch`; on `Error`, it sets `context.error = err`, calls `onError(err, context)`, and marks `isError = true`. If the thrown value is not an `Error`, it is rethrown.
- In [`src/compose.ts`](/Users/goga/Documents/goga/ghx/src/compose.ts), when no handler remains and `context.finalized === false`, `onNotFound` is used instead, so the same composition path can resolve both errors and missing handlers.
- In [`src/hono-base.ts`](/Users/goga/Documents/goga/ghx/src/hono-base.ts), `onError = (handler) => { this.errorHandler = handler }` replaces the active error handler used by dispatch.
- In [`src/hono-base.ts`](/Users/goga/Documents/goga/ghx/src/hono-base.ts), `#handleError(err, c)` returns `this.errorHandler(err, c)` for `Error` objects and rethrows non-`Error` values.
- In [`src/hono-base.ts`](/Users/goga/Documents/goga/ghx/src/hono-base.ts), the multi-handler request path builds `const composed = compose(matchResult[0], this.errorHandler, this.#notFoundHandler)` and awaits it inside a `try/catch` that falls back to `#handleError(err, c)`.
- In [`src/hono-base.ts`](/Users/goga/Documents/goga/ghx/src/hono-base.ts), the single-handler fast path also catches sync and async failures and routes them through `#handleError(...)`.
- In [`src/middleware/combine/index.ts`](/Users/goga/Documents/goga/ghx/src/middleware/combine/index.ts), `every()` reuses `compose(...)` for nested middleware, so the same error propagation semantics apply inside combined middleware chains.

Commands run:
- `ghx read honojs/hono src/compose.ts src/hono-base.ts --lines 1-120 src/hono-base.ts --lines 420-540`
- `ghx read honojs/hono src/hono-base.ts --grep "#handleError\\|onError\\|errorHandler"`

Backends used:
- ghx remote read/grep

Uncertainty:
- I did not trace every individual middleware module; the propagation path is clear at the shared composition layer, but module-specific wrappers could add their own pre/post-processing around it.
- I did not inspect test coverage for these branches, per your constraint.

Suggested next reads:
- `ghx read honojs/hono src/compose.ts --lines 15-70`
- `ghx read honojs/hono src/hono-base.ts --lines 188-230`
- `ghx read honojs/hono src/hono-base.ts --lines 393-457`

Memory reused:
- `compose()` is the shared middleware dispatcher in `src/compose.ts`.
- `HonoBase` calls `compose(matchResult[0], this.errorHandler, this.#notFoundHandler)` in the main request path.
- `HonoBase` has a private `#handleError(err, c)` that delegates to `this.errorHandler(err, c)`.
- `onError()` mutates the active `errorHandler`.
- `middleware/combine/every()` also builds on `compose()`.
