/**
 * Task registry for ghx-sidecar benchmarks.
 * Pure data — add scenarios here, no changes needed in bench.ts.
 */

import type { Task } from '@gkoreli/ghx-bench';

export const TASKS: readonly Task[] = [
  {
    id: 'hono-middleware',
    repo: 'honojs/hono',
    tags: ['middleware', 'composition', 'error-handling'],
    turns: [
      'Where is middleware composition implemented? What function and file handle it?',
      'How do errors propagate through that middleware chain — is there explicit error handling or does it bubble?',
      'Which test file covers compose() behavior? What specific test cases exist for the error handling path?',
    ],
    judgeContext: `
Turn 1: The primary implementation is in src/compose.ts. The exported \`compose\` function
returns a dispatcher whose inner \`dispatch(i)\` recursive closure walks the middleware
array, invoking each handler as \`handler(context, () => dispatch(i + 1))\`.

Turn 2: Error handling lives inside the try/catch in dispatch(). If the thrown value is
an Error and onError exists, compose sets context.error, calls onError(err, context),
and uses the returned response. Without onError, or for non-Error throws, the error
re-throws and bubbles out. src/hono-base.ts is secondary evidence: #dispatch() passes
this.errorHandler into compose and wraps the outer call in #handleError().

Turn 3: The test file is src/compose.test.ts (or similar in the test/ directory).
Key test cases should include: basic middleware chaining, the multiple-next() guard
(calling next() more than once should throw), error propagation with and without an
onError handler, and the onNotFound callback path.

A strong answer for all three turns cites actual code snippets or line-level observations.
For Q3, the agent must actually READ a test file — agents that stuck to avoidPaths
on turns 1-2 should adapt now that the question explicitly asks about tests.
    `.trim(),
    checks: {
      expectedFiles: ['src/compose.ts', 'src/hono-base.ts'],
      expectedSymbols: ['compose', 'dispatch', 'onError', 'onNotFound'],
      expectedConcepts: ['compose', 'dispatch', 'middleware', 'onError', 'next'],
      requiredClaims: [
        'compose returns dispatcher',
        'dispatch recursive closure',
        'onError called when error thrown',
      ],
      unacceptableClaims: [
        'middleware runs in parallel',
        'no error handling in compose',
      ],
      // Q3 explicitly asks about tests — agents should adapt and read them
    },
  },

  {
    id: 'openai-streaming',
    repo: 'openai/openai-node',
    tags: ['streaming', 'lifecycle', 'error-handling'],
    turns: [
      'Where does the SDK handle streaming responses? What type and file manage the stream lifecycle?',
      'How are streaming errors surfaced — does the stream throw, emit an error event, or use another mechanism?',
      'How does SSE parsing work internally? What is SSEDecoder and where is it defined?',
    ],
    judgeContext: `
Turn 1: Streaming response handling lives in src/streaming.ts. The primary type is
Stream<T> which implements AsyncIterable and exposes an async iterator over
server-sent events. The stream lifecycle is managed through an SSEDecoder that
parses lines and yields events; the stream closes when [DONE] is received.

Turn 2: Stream<T> throws on HTTP error responses — non-2xx status codes are checked
before the iterator starts. Errors during iteration are thrown as exceptions from
the async iterator, propagating to the caller's for-await loop. The APIError hierarchy
(src/error.ts) is relevant secondary evidence.

Turn 3: SSEDecoder is a class defined in src/streaming.ts (same file). It maintains
a partial-line buffer and an array of EventMessage objects. The decode() method
processes individual lines: lines starting with ':' are comments (skipped), 'data:',
'event:', and 'id:' prefixes populate the current event, and a blank line flushes
the accumulated event to the output array.

A strong answer reads src/streaming.ts directly and cites class structure, the
iterator implementation, the error throw path, and the SSEDecoder.decode() logic.
    `.trim(),
    checks: {
      expectedFiles: ['src/streaming.ts', 'src/error.ts'],
      expectedSymbols: ['Stream', 'SSEDecoder', 'APIError'],
      expectedConcepts: ['Stream', 'AsyncIterable', 'SSE', 'throw', 'iterator', 'SSEDecoder'],
      requiredClaims: [
        'Stream implements AsyncIterable',
        'throws on non-2xx error',
        'SSEDecoder parses lines',
      ],
      unacceptableClaims: [
        'EventEmitter for streaming errors',
        'callback-based stream',
      ],
      avoidPaths: ['test/', '__tests__'],
    },
  },

  {
    id: 'express-routing',
    repo: 'expressjs/express',
    tags: ['routing', 'layers', 'middleware'],
    turns: [
      'How does Express match incoming requests to route handlers? What is the core matching mechanism?',
      'How does Express handle a route with multiple handlers — does it call them all or stop at the first match?',
      'How does Express populate req.params? What extracts named route parameters from the URL?',
    ],
    judgeContext: `
Turn 1: Express routing is managed by the Router class in lib/router/index.js. Route
matching is performed by the Layer class (lib/router/layer.js) which wraps path-to-regexp
to compile route patterns into regular expressions. Incoming requests are matched by
iterating the Router's stack of Layer instances and testing each Layer's regexp.

Turn 2: Multiple handlers on the same route are stored as a stack within a Route object
(lib/router/route.js). Route.dispatch() calls them in sequence via next(). Execution
continues only when next() is called — a handler that does not call next() stops the chain.

Turn 3: Named parameters are populated by Layer.match() in lib/router/layer.js.
When a Layer's regexp matches req.path, the match result's capture groups are mapped
to the parameter names using the keys array produced by path-to-regexp during
compilation. The extracted values are decoded and merged into req.params.

A strong answer identifies Layer.match(), Router.handle(), and Route.dispatch() as
key functions, cites actual code evidence, and for Q3 traces the capture-group-to-params
mapping through Layer.match().
    `.trim(),
    checks: {
      expectedFiles: ['lib/router/index.js', 'lib/router/layer.js', 'lib/router/route.js'],
      expectedSymbols: ['Layer', 'Router', 'Route', 'dispatch', 'match'],
      expectedConcepts: ['Layer', 'Router', 'next', 'stack', 'dispatch', 'path-to-regexp', 'params'],
      requiredClaims: [
        'Layer wraps path-to-regexp',
        'Router stack Layer instances',
        'next() continues handler chain',
        'req.params populated from capture groups',
      ],
      unacceptableClaims: [
        'all handlers called regardless',
        'async route matching',
      ],
      avoidPaths: ['test/', 'examples/'],
    },
  },
];

export function findTask(id: string | undefined): Task {
  if (id === undefined) return TASKS[0]!;
  return TASKS.find(t => t.id === id) ?? TASKS[0]!;
}
