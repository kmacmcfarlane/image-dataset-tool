# Goa v3 DSL Reference

Complete catalog of DSL functions from `goa.design/goa/v3/dsl`.

Import via dot-import:
```go
import . "goa.design/goa/v3/dsl"
```

## Top-Level Functions

- `API(name, fn)` — Define API with name, description, global properties
- `Service(name, fn)` — Define a service within an API
- `Method(name, fn)` — Define a service method
- `Type(name, args...)` — Define a user type
- `ResultType(identifier, args...)` — Define a result type with views
- `Error(name, args...)` — Describe error return value
- `Server(name, fn...)` — Define server configuration
- `Interceptor(name, fn...)` — Define a named interceptor
- `Host(name, fn)` — Define a server host

## Attribute & Type Functions

- `Attribute(name, args...)` — Describe an object field
- `Field(tag, name, args...)` — Syntactic sugar for Attribute with rpc:tag meta (use for gRPC)
- `Attributes(fn)` — Define result type attributes block
- `ArrayOf(v, fn...)` — Create array type
- `ArrayOfRequired(v, fn...)` — Array type with non-null elements
- `MapOf(k, v, fn...)` — Create map type
- `CollectionOf(v, fn...)` — Create collection result type
- `View(name, fn...)` — Define a view of a result type
- `Extend(t)` — Add parameter type attributes
- `Reference(t)` — Reference another type
- `OneOf(name, args...)` — Mutually exclusive attributes

## Payload & Response

- `Payload(val, args...)` — Define method request payload
- `Result(val, args...)` — Define method response result
- `StreamingPayload(val, args...)` — Streaming request payload
- `StreamingResult(val, args...)` — Streaming response result
- `Response(val, args...)` — HTTP response mapping
- `Body(args...)` — HTTP request/response body

## HTTP Transport

- `HTTP(fns...)` — HTTP transport specific properties
- `GET(path)`, `POST(path)`, `PUT(path)`, `PATCH(path)`, `DELETE(path)`, `HEAD(path)`, `OPTIONS(path)` — HTTP method routes
- `Param(name, args...)` — Query/path parameter
- `Params(args)` — Parameters block
- `Header(name, args...)` — HTTP header
- `Headers(args)` — Headers block
- `Cookie(name, args...)` — HTTP cookie
- `Path(val)` — HTTP path prefix
- `Consumes(args...)` — Request MIME types
- `Produces(args...)` — Response MIME types
- `ContentType(typ)` — Content-Type header
- `Code(code)` — HTTP status code
- `MultipartRequest()` — Multipart form-data
- `Redirect(url, code)` — HTTP redirect
- `Files(path, filename, fns...)` — Static file serving
- `Parent(name)` — Set parent service (URL nesting)
- `CanonicalMethod(name)` — Set canonical method
- `MapParams(args...)` — Map payload to query params
- `SkipRequestBodyEncodeDecode()` — Skip request body encode/decode; service receives `io.ReadCloser` (HTTP only)
- `SkipResponseBodyEncodeDecode()` — Skip response body encode/decode; service returns `io.ReadCloser` (HTTP only)

### HTTP Body Streaming (Skip Encode/Decode)

Use `SkipRequestBodyEncodeDecode()` and `SkipResponseBodyEncodeDecode()` to bypass Goa's generated body encoder/decoder and stream HTTP bodies directly via `io.ReadCloser`. This avoids loading entire request/response bodies into memory.

**Constraints:**
- HTTP transport only — incompatible with gRPC
- Payload/Result attributes must map to headers, path params, or query params (not body)

```go
// Upload: service receives the raw request body as io.ReadCloser
Method("upload", func() {
    Payload(func() {
        Attribute("content_type", String)
        Attribute("dir", String)
    })
    HTTP(func() {
        POST("/upload/{*dir}")
        Header("content_type:Content-Type")
        SkipRequestBodyEncodeDecode()
    })
})

// Download: service returns an io.ReadCloser that is streamed to the client
Method("download", func() {
    Payload(String)
    Result(func() {
        Attribute("length", Int64)
        Required("length")
    })
    HTTP(func() {
        GET("/download/{*filename}")
        SkipResponseBodyEncodeDecode()
        Response(func() {
            Header("length:Content-Length")
        })
    })
})
```

Generated service interface:
```go
type Service interface {
    Upload(context.Context, *UploadPayload, io.ReadCloser) error
    Download(context.Context, string) (*DownloadResult, io.ReadCloser, error)
}
```

### Path Parameters

```go
GET("/users/{user_id}")            // Basic capture
GET("/users/{user_id:id}")         // Map URL param to different payload field
GET("/files/*path")                // Wildcard capture
```

### Service Hierarchies (Nested URLs)

```go
Service("users", func() {
    HTTP(func() {
        Path("/users/{user_id}")
        CanonicalMethod("get")
    })
})
Service("posts", func() {
    Parent("users")  // Inherits /users/{user_id}
    Method("list", func() {
        HTTP(func() {
            GET("/posts")  // Final: /users/{user_id}/posts
        })
    })
})
```

### Server-Sent Events (SSE)

```go
Method("stream", func() {
    StreamingResult(Event)
    HTTP(func() {
        GET("/events/stream")
        ServerSentEvents(func() {
            SSEEventData("message")
            SSEEventType("type")
            SSEEventID("id")
        })
    })
})
```

### WebSocket Streaming (must use GET)

```go
// Bidirectional
Method("echo", func() {
    StreamingPayload(func() { Field(1, "message", String); Required("message") })
    StreamingResult(func() { Field(1, "message", String); Required("message") })
    HTTP(func() { GET("/echo") })
})
```

## gRPC Transport

- `GRPC(fn)` — gRPC transport properties
- `Message(fn)` — gRPC request/response message

Use `Field()` with numeric tags for protobuf field numbers:

```go
Payload(func() {
    Field(1, "a", Int, "Left operand")
    Field(2, "b", Int, "Right operand")
    Required("a", "b")
})
GRPC(func() {
    Response(CodeOK)
})
```

## Security

- `APIKeySecurity(name, fn...)` — API key security scheme
- `BasicAuthSecurity(name, fn...)` — Basic auth security scheme
- `JWTSecurity(name, fn...)` — JWT security scheme
- `OAuth2Security(name, fn...)` — OAuth2 security scheme
- `Security(args...)` — Apply security at API/service/method level
- `NoSecurity()` — Remove security requirement for a method
- `Scope(name, desc...)` — OAuth2/JWT scope
- `AuthorizationCodeFlow(authURL, tokenURL, refreshURL)` — OAuth2 flow
- `ImplicitFlow(authURL, refreshURL)` — OAuth2 implicit flow
- `PasswordFlow(tokenURL, refreshURL)` — OAuth2 password flow
- `ClientCredentialsFlow(tokenURL, refreshURL)` — OAuth2 client credentials
- `APIKey(scheme, name, args...)` — API key payload attribute
- `Username(name, args...)` / `Password(name, args...)` — Basic auth payload attributes
- `AccessToken(name, args...)` — OAuth2 token payload attribute
- `Token(name, args...)` — JWT token payload attribute

## Interceptors

- `Interceptor(name, fn...)` — Define a named interceptor
- `ServerInterceptor(interceptors...)` — Apply server-side interceptors
- `ClientInterceptor(interceptors...)` — Apply client-side interceptors
- `ReadPayload(arg)`, `WritePayload(arg)` — Interceptor payload access
- `ReadResult(arg)`, `WriteResult(arg)` — Interceptor result access
- `ReadStreamingPayload(arg)`, `WriteStreamingPayload(arg)` — Streaming payload access
- `ReadStreamingResult(arg)`, `WriteStreamingResult(arg)` — Streaming result access

```go
var Cache = Interceptor("Cache", func() {
    ReadPayload(func() {
        Attribute("recordID", String)
    })
    WriteResult(func() {
        Attribute("cachedAt", String)
        Attribute("ttl", Int)
    })
})

Service("catalog", func() {
    ServerInterceptor(Cache)
})
```

Request flow: HTTP Request -> HTTP Middleware -> Goa Transport -> Goa Interceptors -> Service Method

## Validation

- `Required(names...)` — Mark attributes as required
- `Enum(vals...)` — Enum validation
- `Pattern(p)` — Regex pattern validation
- `MinLength(val)`, `MaxLength(val)` — String/array length
- `Minimum(val)`, `Maximum(val)` — Numeric range
- `ExclusiveMinimum(val)`, `ExclusiveMaximum(val)` — Exclusive range
- `Format(f)` — Format validation
- `Elem(fn)` — Array/map element validations
- `Default(def)` — Set default value

### Validation Formats

`FormatDate`, `FormatDateTime`, `FormatRFC1123`, `FormatUUID`, `FormatEmail`, `FormatHostname`, `FormatIPv4`, `FormatIPv6`, `FormatIP`, `FormatURI`, `FormatMAC`, `FormatCIDR`, `FormatRegexp`, `FormatJSON`

## Documentation

- `Title(val)`, `Description(d)`, `Docs(fn)`, `Version(ver)`
- `TermsOfService(terms)`, `Contact(fn)`, `License(fn)`
- `Name(name)`, `Email(email)`, `URL(url)`
- `Example(args...)` — Example values for documentation/testing

## Metadata & Conversion

- `Meta(name, value...)` — Set metadata on design elements
- `Tag(name, value)` — Set metadata tag
- `ConvertTo(obj)` — External type conversion
- `CreateFrom(obj)` — External type initialization
- `TypeName(name)` — Override generated type name

### Useful Meta Keys

- `Meta("struct:tag:json", "SSN,omitempty")` — Custom struct tags
- `Meta("struct:tag:xml", "SSN,omitempty")` — XML tags
- `Meta("rpc:tag", "1")` — Protobuf field number (alternative to `Field()`)
- `Meta("type:generate:force")` — Force generation of unreferenced types
- `Meta("struct:error:name")` — Required on custom error types for disambiguation
- `Meta("openapi:generate", "false")` — Disable OpenAPI generation

## Primitive Types

`Boolean`, `Int`, `Int32`, `Int64`, `UInt`, `UInt32`, `UInt64`, `Float32`, `Float64`, `String`, `Bytes`, `Any`, `Empty`

## HTTP Status Codes

`StatusOK` (200), `StatusCreated` (201), `StatusAccepted` (202), `StatusNoContent` (204), `StatusBadRequest` (400), `StatusUnauthorized` (401), `StatusForbidden` (403), `StatusNotFound` (404), `StatusConflict` (409), `StatusInternalServerError` (500)

## gRPC Codes

`CodeOK`, `CodeCanceled`, `CodeUnknown`, `CodeInvalidArgument`, `CodeDeadlineExceeded`, `CodeNotFound`, `CodeAlreadyExists`, `CodePermissionDenied`, `CodeResourceExhausted`, `CodeFailedPrecondition`, `CodeUnimplemented`, `CodeInternal`

## Plugins

Import path: `goa.design/plugins/v3`

- `cors` — CORS policy DSL and generated middleware
- `docs` — Documentation generation
- `goakit` — go-kit library integration
- `i18n` — Internationalization
- `otel` — OpenTelemetry integration
- `zaplogger` — Zap logger integration
- `zerologger` — Zerolog integration

### CORS Example

```go
import cors "goa.design/plugins/v3/cors/dsl"

API("calc", func() {
    cors.Origin("http://127.0.0.1", func() {
        cors.Headers("X-Shared-Secret")
        cors.Methods("GET", "POST")
        cors.Expose("X-Time")
        cors.MaxAge(600)
        cors.Credentials()
    })
})
```
