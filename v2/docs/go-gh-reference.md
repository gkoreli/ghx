# go-gh v2 API Reference

Distilled reference for `github.com/cli/go-gh/v2` — minimal working examples and gotchas for agents.

## API Surface

### Core Clients

- `api.DefaultGraphQLClient()` — GraphQL client with auto-resolved auth (GH_TOKEN, GH_HOST, stored OAuth)
- `api.NewGraphQLClient(opts ClientOptions)` — GraphQL client with explicit options
- `api.DefaultRESTClient()` — REST client with auto-resolved auth
- `api.NewRESTClient(opts ClientOptions)` — REST client with explicit options
- `api.DefaultHTTPClient()` — Raw HTTP client (rarely needed)
- `gh.Exec(args ...string)` — Shell out to `gh` CLI, returns (stdout, stderr, error)
- `gh.ExecContext(ctx, args ...)` — Exec with context
- `gh.ExecInteractive(ctx, args ...)` — Exec with interactive terminal
- `repository.Current()` — Get current repo from GH_REPO env or git remote

### Error Types

- `api.HTTPError` — REST API errors; fields: `StatusCode`, `Message`, `Errors[]`, `Headers`, `RequestURL`
- `api.GraphQLError` — GraphQL errors; fields: `Errors[]` (each has `Message`, `Path`, `Type`, `Extensions`)
- `api.HTTPError.Error()` — Formats as "HTTP 404: Not Found (url)"
- `api.GraphQLError.Error()` — Formats as "GraphQL: message (path), message (path)"
- `api.GraphQLError.Match(expectType, expectPath)` — Check if error matches type/path (path can end with "." for prefix match)

### ClientOptions

```go
type ClientOptions struct {
    Host        string                 // github.com or enterprise host
    AuthToken   string                 // explicit token (overrides env/config)
    Headers     map[string]string      // custom headers
    Timeout     time.Duration          // request timeout
    EnableCache bool                   // cache GraphQL responses
    Log         io.Writer              // log HTTP requests/responses
}
```

## Auth Model

**Resolution order** (first match wins):

1. `ClientOptions.AuthToken` if provided
2. `GH_TOKEN` env var
3. `GH_HOST` env var (for host selection)
4. Stored OAuth token from `gh` config (~/.config/gh/hosts.yml)
5. Default host: github.com

**Key insight**: `DefaultGraphQLClient()` and `DefaultRESTClient()` call `NewGraphQLClient(ClientOptions{})` and `NewRESTClient(ClientOptions{})` — empty options trigger full resolution chain.

**Gotcha**: If `GH_TOKEN` is set but invalid, auth will fail silently at request time, not at client creation.

## GraphQL Usage

### Query Pattern

```go
client, err := api.DefaultGraphQLClient()
var query struct {
    Repository struct {
        Name string
    } `graphql:"repository(owner: $owner, name: $name)"`
}
variables := map[string]interface{}{
    "owner": graphql.String("cli"),
    "name":  graphql.String("cli"),
}
err = client.Query("RepositoryName", &query, variables)
```

**Methods**:
- `Query(name string, q interface{}, variables map[string]interface{}) error` — Execute query, decode into struct
- `QueryWithContext(ctx, name, q, variables)` — Query with context
- `Mutate(name string, m interface{}, variables map[string]interface{}) error` — Execute mutation
- `MutateWithContext(ctx, name, m, variables)` — Mutate with context
- `Do(query string, variables map[string]interface{}, response interface{}) error` — Raw query string (rarely used)
- `DoWithContext(ctx, query, variables, response)` — Do with context

**When to use each**:
- `Query()` — Standard queries, struct-based response
- `Mutate()` — Mutations (create, update, delete)
- `Do()` — Only if you have raw query string and can't use Query/Mutate

**Gotcha**: Query name (first arg) is for logging/caching only, not sent to API. Actual query is in struct tags.

### Pagination Pattern

```go
variables := map[string]interface{}{
    "endCursor": (*graphql.String)(nil),  // nil for first page
}
for {
    err := client.Query("Releases", &query, variables)
    if !query.Repository.Releases.PageInfo.HasNextPage {
        break
    }
    variables["endCursor"] = graphql.String(query.Repository.Releases.PageInfo.EndCursor)
}
```

**Key**: Use `PageInfo.HasNextPage` and `PageInfo.EndCursor` to iterate. Set cursor to nil for first page.

## REST Usage

### Get Pattern

```go
client, err := api.DefaultRESTClient()
response := []struct{ Name string }{}
err = client.Get("repos/cli/cli/tags", &response)
```

**Methods**:
- `Get(path string, resp interface{}) error` — GET, decode JSON into resp
- `Post(path string, body io.Reader, resp interface{}) error` — POST with body
- `Patch(path string, body io.Reader, resp interface{}) error` — PATCH
- `Put(path string, body io.Reader, resp interface{}) error` — PUT
- `Delete(path string, resp interface{}) error` — DELETE
- `Do(method, path string, body io.Reader, resp interface{}) error` — Generic method
- `Request(method, path string, body io.Reader) (*http.Response, error)` — Raw response (for streaming, custom parsing)

**Query params**: Append to path as query string: `"repos/cli/cli/issues?state=open&per_page=10"`

**Custom headers**: Pass via `ClientOptions.Headers` at client creation time.

**Gotcha**: `Request()` returns raw `*http.Response` — caller must close `Body`. Use `Do()` for auto-parsing.

### Pagination Pattern

```go
linkRE := regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)"`)
findNextPage := func(resp *http.Response) (string, bool) {
    for _, m := range linkRE.FindAllStringSubmatch(resp.Header.Get("Link"), -1) {
        if len(m) > 2 && m[2] == "next" {
            return m[1], true
        }
    }
    return "", false
}
path := "repos/cli/cli/releases"
for {
    resp, err := client.Request(http.MethodGet, path, nil)
    data := []struct{ Name string }{}
    json.NewDecoder(resp.Body).Decode(&data)
    resp.Body.Close()
    
    if path, ok := findNextPage(resp); !ok {
        break
    }
}
```

**Key**: REST pagination uses `Link` header with `rel="next"`. Parse it to get next URL.

## Error Handling

### HTTPError

```go
client, err := api.DefaultRESTClient()
err = client.Get("repos/cli/cli/tags", &response)
if err != nil {
    if httpErr, ok := err.(*api.HTTPError); ok {
        if httpErr.StatusCode == 404 {
            // not found
        }
        // httpErr.Message, httpErr.Errors[], httpErr.Headers available
    }
}
```

**Fields**:
- `StatusCode` — HTTP status (404, 403, 500, etc.)
- `Message` — Human-readable error message
- `Errors[]` — Structured errors (each has Code, Field, Resource, Message)
- `Headers` — Response headers
- `RequestURL` — URL that failed

### GraphQLError

```go
err = client.Query("Releases", &query, variables)
if err != nil {
    if gqlErr, ok := err.(*api.GraphQLError); ok {
        if gqlErr.Match("AUTHENTICATION_ERROR", "viewer.") {
            // auth failed on viewer query
        }
        for _, e := range gqlErr.Errors {
            // e.Message, e.Path, e.Type, e.Extensions
        }
    }
}
```

**Match(expectType, expectPath)**:
- `expectPath` can end with "." to match prefix (e.g., "viewer." matches "viewer.login", "viewer.repositories")
- Returns true if ALL errors match the type and path
- Use for checking specific error conditions

## gh.Exec() — Shell Out

```go
stdout, stderr, err := gh.Exec("issue", "list", "-R", "cli/cli", "--limit", "5")
if err != nil {
    // err is exit code error
}
fmt.Println(stdout.String())
```

**When to use**:
- Complex `gh` commands not available via API (e.g., `gh workflow run`)
- Leverage existing `gh` plugins
- One-off operations where API overhead isn't worth it

**Gotcha**: Returns `bytes.Buffer` for stdout/stderr, not strings. Call `.String()` to convert.

## Gotchas & Anti-Patterns

1. **Auth fails silently** — Invalid token won't error until first request. Always check first request error.

2. **GraphQL query name is metadata** — The first arg to `Query()` is for logging/caching, not sent to API. Actual query is in struct tags.

3. **Request() doesn't auto-close** — If using `Request()` for raw response, you must close `Body`. Use `Do()` for auto-parsing.

4. **Pagination cursors are opaque** — Don't parse or modify GraphQL cursors. Pass them as-is to next request.

5. **REST query params in path** — No helper for query params. Append to path string: `"repos/cli/cli/issues?state=open"`

6. **Rate limits not exposed** — Check `X-RateLimit-*` headers manually if needed. No built-in rate limit handling.

7. **Timeout applies to entire request** — `ClientOptions.Timeout` is request-level, not per-operation. Large responses may timeout.

8. **Cache is per-client** — `EnableCache: true` caches within one client instance. Different clients don't share cache.

9. **Headers are set at client creation** — Can't change headers per-request. Create new client if headers differ.

10. **GH_HOST selects host, not auth** — `GH_HOST=github.enterprise.com` tells go-gh which host to use, but token still comes from GH_TOKEN or config.

## Code Examples

### GraphQL Query with Error Handling

```go
client, err := api.DefaultGraphQLClient()
if err != nil {
    log.Fatal(err)
}

var query struct {
    Repository struct {
        Name string
    } `graphql:"repository(owner: $owner, name: $name)"`
}

err = client.Query("GetRepo", &query, map[string]interface{}{
    "owner": graphql.String("cli"),
    "name":  graphql.String("cli"),
})

if err != nil {
    if gqlErr, ok := err.(*api.GraphQLError); ok {
        log.Printf("GraphQL error: %v", gqlErr.Error())
    } else {
        log.Fatal(err)
    }
}
```

### REST GET with Query Params

```go
client, err := api.DefaultRESTClient()
if err != nil {
    log.Fatal(err)
}

response := []struct{ Name string }{}
err = client.Get("repos/cli/cli/issues?state=open&per_page=10", &response)
if err != nil {
    if httpErr, ok := err.(*api.HTTPError); ok {
        log.Printf("HTTP %d: %s", httpErr.StatusCode, httpErr.Message)
    } else {
        log.Fatal(err)
    }
}
```

### REST Pagination

```go
client, err := api.DefaultRESTClient()
path := "repos/cli/cli/releases?per_page=30"

for {
    resp, err := client.Request(http.MethodGet, path, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    data := []struct{ Name string }{}
    json.NewDecoder(resp.Body).Decode(&data)
    resp.Body.Close()
    
    // Check for next page
    nextPath := ""
    for _, link := range strings.Split(resp.Header.Get("Link"), ",") {
        if strings.Contains(link, `rel="next"`) {
            // Extract URL from <url>; rel="next"
            parts := strings.Split(link, ";")
            nextPath = strings.Trim(strings.TrimSpace(parts[0]), "<>")
            break
        }
    }
    
    if nextPath == "" {
        break
    }
    path = nextPath
}
```

### Custom Client with Timeout

```go
opts := api.ClientOptions{
    Host:        "github.com",
    AuthToken:   os.Getenv("GH_TOKEN"),
    Timeout:     10 * time.Second,
    EnableCache: true,
}
client, err := api.NewGraphQLClient(opts)
if err != nil {
    log.Fatal(err)
}
```
