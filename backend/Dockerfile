FROM golang:1.25-alpine AS builder

RUN go install goa.design/goa/v3/cmd/goa@latest

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go generate ./internal/api/...
RUN go build -o server ./cmd/server

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /build/server .
COPY --from=builder /build/internal/api/design/public ./public
COPY --from=builder /build/internal/api/gen/http/openapi3.json ./gen/http/openapi3.json

RUN mkdir -p /app/data
EXPOSE 8080
CMD ["./server"]
