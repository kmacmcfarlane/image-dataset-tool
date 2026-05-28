FROM golang:1.25-alpine AS builder

# goheif (libde265 + dav1d) requires CGO; install C/C++ build tools for alpine.
RUN apk add --no-cache gcc g++ musl-dev

RUN go install goa.design/goa/v3/cmd/goa@latest

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go generate ./internal/api/...
RUN CGO_ENABLED=1 go build -o server ./cmd/server

FROM alpine:3.21
# libstdc++ required at runtime by goheif (libde265 C++ code).
RUN apk add --no-cache libstdc++
WORKDIR /app
COPY --from=builder /build/server .
COPY --from=builder /build/internal/api/design/public ./public
COPY --from=builder /build/internal/api/gen/http/openapi3.json ./gen/http/openapi3.json

RUN mkdir -p /app/data
EXPOSE 8080
CMD ["./server"]
