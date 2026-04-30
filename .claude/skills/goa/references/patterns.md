# Goa v3 Design Patterns

## Complete Service Example

```go
package design

import . "goa.design/goa/v3/dsl"

var _ = API("calc", func() {
    Title("Calculator Service")
    Description("HTTP service for multiplying numbers")
    Server("calc", func() {
        Description("calc hosts the Calculator Service.")
        Services("calc")
        Host("development", func() {
            Description("Development hosts.")
            URI("http://localhost:8000/calc")
            URI("grpc://localhost:8080")
        })
        Host("production", func() {
            Description("Production hosts.")
            URI("https://{version}.goa.design/calc")
            URI("grpcs://{version}.goa.design")
            Variable("version", String, "API version", func() {
                Default("v1")
            })
        })
    })
})

var _ = Service("calc", func() {
    Description("The calc service performs operations on numbers")
    Method("multiply", func() {
        Payload(func() {
            Attribute("a", Int, "Left operand", func() {
                Meta("rpc:tag", "1")
            })
            Field(2, "b", Int, "Right operand")
            Required("a", "b")
        })
        Result(Int)
        HTTP(func() {
            GET("/multiply/{a}/{b}")
            Response(StatusOK)
        })
        GRPC(func() {
            Response(CodeOK)
        })
    })
    Files("/openapi.json", "gen/http/openapi3.json")
})
```

## CRUD Service Pattern

```go
var _ = Service("users", func() {
    Description("User management service")

    Method("list", func() {
        Payload(func() {
            Attribute("page", Int, "Page number", func() {
                Default(1)
                Minimum(1)
            })
            Attribute("per_page", Int, "Items per page", func() {
                Default(20)
                Minimum(1)
                Maximum(100)
            })
        })
        Result(CollectionOf(UserResult))
        HTTP(func() {
            GET("/users")
            Param("page")
            Param("per_page")
            Response(StatusOK)
        })
    })

    Method("get", func() {
        Payload(func() {
            Attribute("id", String, "User ID", func() {
                Format(FormatUUID)
            })
            Required("id")
        })
        Result(UserResult)
        Error("not_found", ErrorResult, "User not found")
        HTTP(func() {
            GET("/users/{id}")
            Response(StatusOK)
            Response("not_found", StatusNotFound)
        })
    })

    Method("create", func() {
        Payload(func() {
            Attribute("name", String, "User name", func() {
                MinLength(1)
                MaxLength(255)
            })
            Attribute("email", String, "Email address", func() {
                Format(FormatEmail)
            })
            Required("name", "email")
        })
        Result(UserResult)
        Error("already_exists", ErrorResult, "User already exists")
        HTTP(func() {
            POST("/users")
            Response(StatusCreated)
            Response("already_exists", StatusConflict)
        })
    })

    Method("update", func() {
        Payload(func() {
            Attribute("id", String, "User ID", func() {
                Format(FormatUUID)
            })
            Attribute("name", String, "User name", func() {
                MinLength(1)
                MaxLength(255)
            })
            Attribute("email", String, "Email address", func() {
                Format(FormatEmail)
            })
            Required("id")
        })
        Result(UserResult)
        Error("not_found", ErrorResult, "User not found")
        HTTP(func() {
            PUT("/users/{id}")
            Response(StatusOK)
            Response("not_found", StatusNotFound)
        })
    })

    Method("delete", func() {
        Payload(func() {
            Attribute("id", String, "User ID", func() {
                Format(FormatUUID)
            })
            Required("id")
        })
        Error("not_found", ErrorResult, "User not found")
        HTTP(func() {
            DELETE("/users/{id}")
            Response(StatusNoContent)
            Response("not_found", StatusNotFound)
        })
    })
})

var UserResult = ResultType("application/vnd.user", "UserResult", func() {
    Attributes(func() {
        Attribute("id", String, "User ID", func() {
            Format(FormatUUID)
        })
        Attribute("name", String, "User name")
        Attribute("email", String, "Email address")
        Attribute("created_at", String, "Created timestamp", func() {
            Format(FormatDateTime)
        })
        Required("id", "name", "email", "created_at")
    })
    View("default", func() {
        Attribute("id")
        Attribute("name")
        Attribute("email")
        Attribute("created_at")
    })
    View("tiny", func() {
        Attribute("id")
        Attribute("name")
    })
})
```

## Error Handling

### Defining Errors at Three Levels

```go
// API-level (shared across all services)
var _ = API("myapi", func() {
    Error("unauthorized")
    HTTP(func() {
        Response("unauthorized", StatusUnauthorized)
    })
})

// Service-level (shared across all methods in service)
var _ = Service("users", func() {
    Error("invalid_arguments", ErrorResult, "Invalid arguments")
})

// Method-level
Method("divide", func() {
    Error("div_by_zero")
})
```

### Custom Error Types

```go
var DivByZero = Type("DivByZero", func() {
    Field(1, "message", String, "Error message")
    Field(2, "dividend", Int, "Dividend value")
    Field(3, "name", String, "Error name", func() {
        Meta("struct:error:name")  // Required for multiple custom errors
    })
    Required("message", "dividend", "name")
})

Method("divide", func() {
    Error("DivByZero", DivByZero, "Division by zero")
    HTTP(func() {
        Response("DivByZero", StatusBadRequest)
    })
})
```

### Error Properties

```go
Error("service_unavailable", ErrorResult, func() { Temporary() })  // Client should retry
Error("request_timeout", ErrorResult, func() { Timeout() })        // Deadline exceeded
Error("internal_error", ErrorResult, func() { Fault() })           // Server-side problem
```

### Producing Errors in Implementation

```go
func (s *dividerSvc) IntegralDivide(ctx context.Context, p *divider.IntOperands) (int, error) {
    if p.Divisor == 0 {
        return 0, gendivider.MakeDivByZero(fmt.Errorf("divisor cannot be zero"))
    }
    return p.Dividend / p.Divisor, nil
}
```

### Consuming Errors (Client-Side)

```go
res, err := client.Divide(ctx, payload)
if serr, ok := err.(*goa.ServiceError); ok {
    switch serr.Name {
    case "DivByZero":
        // Handle division by zero
    }
}
```

## Security Patterns

### JWT Authentication

```go
var JWT = JWTSecurity("jwt", func() {
    Scope("api:read", "Read access")
    Scope("api:write", "Write access")
})

var _ = Service("secure", func() {
    Security(JWT)

    Method("list", func() {
        Security(JWT, func() {
            Scope("api:read")
        })
        Payload(func() {
            Token("token", String)
            Required("token")
        })
        Result(CollectionOf(ItemResult))
        HTTP(func() {
            GET("/items")
        })
    })

    // Public endpoint
    Method("health", func() {
        NoSecurity()
        Result(String)
        HTTP(func() {
            GET("/health")
        })
    })
})
```

### Basic Auth

```go
var BasicAuth = BasicAuthSecurity("basicauth", func() {
    Description("Use your credentials")
})

Method("login", func() {
    Security(BasicAuth)
    Payload(func() {
        Username("user", String)
        Password("pass", String)
    })
    HTTP(func() {
        POST("/login")
    })
})
```

### API Key

```go
var APIKeyAuth = APIKeySecurity("api_key", func() {
    Description("API key authentication")
})

Method("secure_endpoint", func() {
    Security(APIKeyAuth)
    Payload(func() {
        APIKey("api_key", "key", String, "API key")
        Required("key")
    })
    HTTP(func() {
        GET("/secure")
        Param("key:key")  // Send as query parameter
        // Or: Header("key:X-API-Key")  // Send as header
    })
})
```

### OAuth2

```go
var OAuth2 = OAuth2Security("oauth2", func() {
    AuthorizationCodeFlow("/authorize", "/token", "/refresh")
    Scope("api:read", "Read access")
    Scope("api:write", "Write access")
})
```

## ResultType with Views

```go
var BottleResult = ResultType("application/vnd.bottle", "BottleResult", func() {
    Description("A bottle of wine")
    Attributes(func() {
        Attribute("id", Int, "ID of bottle")
        Attribute("name", String, "Name of bottle")
        Attribute("vintage", Int, "Vintage year")
        Attribute("account", AccountResult, "Owner account")
        Required("id", "name")
    })
    View("default", func() {
        Attribute("id")
        Attribute("name")
        Attribute("vintage")
    })
    View("extended", func() {
        Attribute("id")
        Attribute("name")
        Attribute("vintage")
        Attribute("account")
    })
    View("tiny", func() {
        Attribute("id")
        Attribute("name")
    })
})

// Collection with specific view
var TinyBottles = CollectionOf(BottleResult, func() {
    View("tiny")
})
```

## Payload with Validation

```go
Payload(func() {
    Attribute("username", String, "Username", func() {
        Pattern("^[a-zA-Z][a-zA-Z0-9_]{2,31}$")
        MinLength(3)
        MaxLength(32)
    })
    Attribute("email", String, "Email address", func() {
        Format(FormatEmail)
    })
    Attribute("age", Int, "User age", func() {
        Minimum(0)
        Maximum(150)
    })
    Attribute("role", String, "User role", func() {
        Enum("admin", "user", "moderator")
        Default("user")
    })
    Attribute("tags", ArrayOf(String), "User tags", func() {
        MinLength(1)
        MaxLength(10)
    })
    Required("username", "email")
})
```

## Multi-Transport (HTTP + gRPC)

```go
Method("add", func() {
    Payload(func() {
        Field(1, "a", Int, "Left operand")
        Field(2, "b", Int, "Right operand")
        Required("a", "b")
    })
    Result(Int)
    HTTP(func() {
        POST("/add")
        Response(StatusOK)
    })
    GRPC(func() {
        Response(CodeOK)
    })
})
```

Use `Field()` instead of `Attribute()` when supporting gRPC — the numeric tag becomes the protobuf field number.

## Service Implementation Pattern

```go
package myapi

import (
    "context"
    "log"
    gen "mymodule/gen/myservice"
)

type myservicesrvc struct {
    logger *log.Logger
}

func NewMyService(logger *log.Logger) gen.Service {
    return &myservicesrvc{logger}
}

func (s *myservicesrvc) List(ctx context.Context, p *gen.ListPayload) (*gen.ListResult, error) {
    s.logger.Printf("myservice.list called with page=%d", p.Page)
    // Your business logic here
    return &gen.ListResult{}, nil
}
```

## HTTP Handler Wireup (Preferred)

The Goa HTTP transport layer (mux, servers, mounting, middleware) MUST be extracted into a `NewHTTPHandler()` function in `internal/api/http.go`. Do NOT inline this plumbing in `main.go`.

### Separation of concerns

- **`main.go`** owns dependency injection: config → stores → services → API service impls → Goa endpoints (+ endpoint middleware). It calls `NewHTTPHandler()` and gets back an `http.Handler`.
- **`internal/api/http.go`** owns HTTP transport: mux creation, decoder/encoder, server instantiation, error handler, mounting, mount logging, debug middleware, and HTTP-level middleware (RequestID, Log, CORS, etc.).
- **Boundary**: `main.go` passes `*<service>.Endpoints` in, gets `http.Handler` back. All Goa HTTP plumbing is encapsulated.

### internal/api/http.go

```go
package api

import (
    "context"
    "log"
    "net/http"
    "os"

    "mymodule/gen/health"
    "mymodule/gen/users"
    healthsvr "mymodule/gen/http/health/server"
    userssvr "mymodule/gen/http/users/server"
    goahttp "goa.design/goa/v3/http"
    httpmdlwr "goa.design/goa/v3/http/middleware"
    "goa.design/goa/v3/middleware"
)

// NewHTTPHandler returns a standard http.Handler with all Goa services
// mounted and middleware applied.
func NewHTTPHandler(
    usersEndpoints *users.Endpoints,
    healthEndpoints *health.Endpoints,
    logger *log.Logger,
    debug bool,
) http.Handler {

    // Setup goa log adapter.
    var (
        adapter middleware.Logger
    )
    {
        adapter = middleware.NewLogger(logger)
    }

    // Transport-specific request decoder and response encoder.
    var (
        dec = goahttp.RequestDecoder
        enc = goahttp.ResponseEncoder
    )

    // Build the service HTTP request multiplexer.
    mux := goahttp.NewMuxer()

    // Create servers for each service.
    var (
        usersServer  *userssvr.Server
        healthServer *healthsvr.Server
    )
    {
        eh := errorHandler(logger)
        usersServer = userssvr.New(usersEndpoints, mux, dec, enc, eh, nil)
        healthServer = healthsvr.New(healthEndpoints, mux, dec, enc, eh, nil)
        // Debug middleware logs HTTP requests and responses (headers + bodies)
        // to stdout. Only enable in development.
        if debug {
            servers := goahttp.Servers{
                usersServer,
                healthServer,
            }
            servers.Use(httpmdlwr.Debug(mux, os.Stdout))
        }
    }

    // Mount servers on the mux and log routes.
    userssvr.Mount(mux, usersServer)
    for _, m := range usersServer.Mounts {
        logger.Printf("HTTP %q mounted on %s %s", m.Method, m.Verb, m.Pattern)
    }

    healthsvr.Mount(mux, healthServer)
    for _, m := range healthServer.Mounts {
        logger.Printf("HTTP %q mounted on %s %s", m.Method, m.Verb, m.Pattern)
    }

    // Wrap with HTTP middleware (applies to all endpoints).
    var handler http.Handler = mux
    {
        handler = httpmdlwr.Log(adapter)(handler)
        handler = httpmdlwr.RequestID()(handler)
    }

    return handler
}

// errorHandler returns a function that writes and logs the given error
// along with the request ID for correlation.
func errorHandler(logger *log.Logger) func(context.Context, http.ResponseWriter, error) {
    return func(ctx context.Context, w http.ResponseWriter, err error) {
        id := ctx.Value(middleware.RequestIDKey).(string)
        _, _ = w.Write([]byte("[" + id + "] encoding: " + err.Error()))
        logger.Printf("[%s] ERROR: %s", id, err.Error())
    }
}
```

### main.go (relevant wireup portion)

```go
// Create service implementations.
var (
    usersSvc  users.Service
    healthSvc health.Service
)
{
    usersSvc = api.NewUsersService(userStore, logger)
    healthSvc = api.NewHealthService()
}

// Wrap services in Goa endpoints.
var (
    usersEndpoints  *users.Endpoints
    healthEndpoints *health.Endpoints
)
{
    usersEndpoints = users.NewEndpoints(usersSvc)
    healthEndpoints = health.NewEndpoints(healthSvc)
}

// Build the HTTP handler (all Goa transport setup is encapsulated here).
httpHandler := api.NewHTTPHandler(
    usersEndpoints,
    healthEndpoints,
    logger,
    debug,
)

// Create and start the HTTP server.
srv := &http.Server{
    Addr:    addr,
    Handler: httpHandler,
}
```

## HTTP Body Streaming (Upload/Download)

Use `SkipRequestBodyEncodeDecode()` and `SkipResponseBodyEncodeDecode()` to stream HTTP bodies directly via `io.ReadCloser`, avoiding loading entire payloads into memory. HTTP only — incompatible with gRPC.

### Design

```go
var _ = Service("updown", func() {
    Description("File upload and download with streaming.")

    Method("upload", func() {
        // Payload attributes must map to headers or path params — not body.
        Payload(func() {
            Attribute("content_type", String, "Content-Type header, must define value for multipart boundary.", func() {
                Default("multipart/form-data; boundary=goa")
                Pattern("multipart/[^;]+; boundary=.+")
            })
            Attribute("dir", String, "Upload directory.", func() {
                Default("upload")
            })
        })
        Error("invalid_media_type", ErrorResult)
        Error("invalid_multipart_request", ErrorResult)
        Error("internal_error", ErrorResult)
        HTTP(func() {
            POST("/upload/{*dir}")
            Header("content_type:Content-Type")
            SkipRequestBodyEncodeDecode()
            Response("invalid_media_type", StatusBadRequest)
            Response("invalid_multipart_request", StatusBadRequest)
            Response("internal_error", StatusInternalServerError)
        })
    })

    Method("download", func() {
        Payload(String, func() {
            Description("Path to downloaded file.")
        })
        // Result attributes must map to headers — not body.
        Result(func() {
            Attribute("length", Int64, "Content length in bytes.")
            Required("length")
        })
        Error("invalid_file_path", ErrorResult)
        Error("internal_error", ErrorResult)
        HTTP(func() {
            GET("/download/{*filename}")
            SkipResponseBodyEncodeDecode()
            Response(func() {
                Header("length:Content-Length")
            })
            Response("invalid_file_path", StatusNotFound)
            Response("internal_error", StatusInternalServerError)
        })
    })
})
```

### Generated Service Interface

```go
type Service interface {
    // Upload receives the raw HTTP request body as io.ReadCloser.
    Upload(context.Context, *UploadPayload, io.ReadCloser) error
    // Download returns a result (mapped to headers) and an io.ReadCloser
    // that is streamed as the response body.
    Download(context.Context, string) (*DownloadResult, io.ReadCloser, error)
}
```

### Implementation

```go
// Upload streams request body to disk using multipart reader.
func (s *updownsrvc) Upload(ctx context.Context, p *updown.UploadPayload, body io.ReadCloser) error {
    defer body.Close()

    uploadDir := filepath.Join(s.dir, p.Dir)
    if err := os.MkdirAll(uploadDir, 0777); err != nil {
        return updown.MakeInternalError(err)
    }

    _, params, err := mime.ParseMediaType(p.ContentType)
    if err != nil {
        return updown.MakeInvalidMediaType(err)
    }
    mr := multipart.NewReader(body, params["boundary"])

    for {
        part, err := mr.NextPart()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return updown.MakeInvalidMultipartRequest(err)
        }
        f, err := os.Create(filepath.Join(uploadDir, part.FileName()))
        if err != nil {
            return updown.MakeInternalError(err)
        }
        defer f.Close()
        if _, err := io.Copy(f, part); err != nil {
            return updown.MakeInternalError(err)
        }
    }
}

// Download streams a file from disk to the client.
func (s *updownsrvc) Download(ctx context.Context, filename string) (*updown.DownloadResult, io.ReadCloser, error) {
    abs := filepath.Join(s.dir, filename)
    fi, err := os.Stat(abs)
    if err != nil {
        return nil, nil, updown.MakeInvalidFilePath(err)
    }
    f, err := os.Open(abs)
    if err != nil {
        return nil, nil, updown.MakeInternalError(err)
    }
    // Goa streams f to the client and closes it when done.
    return &updown.DownloadResult{Length: fi.Size()}, f, nil
}
```

### Key Rules

- **Payload/Result attributes cannot be body fields** — they must map to headers, path params, or query params
- **Always close the `io.ReadCloser`** in upload handlers (`defer body.Close()`)
- **Return an `io.ReadCloser`** from download handlers — Goa handles streaming and closing it
- **HTTP only** — these DSL functions are incompatible with gRPC transport

## Static Files

```go
Service("web", func() {
    Files("/static/{*path}", "./public")
    Files("/favicon.ico", "./public/favicon.ico")
    Files("/openapi.json", "gen/http/openapi3.json")
})
```

## Interceptors

```go
var Logging = Interceptor("Logging", func() {
    ReadPayload(func() {
        Attribute("id", String)
    })
})

var _ = Service("catalog", func() {
    ServerInterceptor(Logging)
    // methods...
})
```

Interceptors vs HTTP middleware:
- HTTP middleware: standard `http.Handler` pattern for transport concerns (CORS, compression)
- Goa interceptors: type-safe access to domain types, compile-time checked
