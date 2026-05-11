package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = API("imagedatasettool", func() {
	Title("Image Dataset Tool API")
	Description("API for managing image datasets for Flux LoRA training")
	Server("imagedatasettool", func() {
		Host("localhost", func() {
			URI("http://localhost:8080")
		})
	})
})

// ---------------------------------------------------------------------------
// Health service — enhanced with subsystem status
// ---------------------------------------------------------------------------

var _ = Service("health", func() {
	Description("Health check service")

	Method("check", func() {
		Description("Check system health with subsystem status")
		Result(HealthResult)
		Error("unhealthy", HealthResult, "One or more subsystems are unhealthy")
		HTTP(func() {
			GET("/health")
			Response(StatusOK)
			Response("unhealthy", StatusServiceUnavailable)
		})
	})
})

var HealthResult = Type("HealthResult", func() {
	Attribute("status", String, "Overall health status (healthy or unhealthy)", func() {
		Enum("healthy", "unhealthy")
		Example("healthy")
	})
	Attribute("subsystems", MapOf(String, SubsystemStatus), "Per-subsystem health status")
	Required("status", "subsystems")
})

var SubsystemStatus = Type("SubsystemStatus", func() {
	Attribute("ok", Boolean, "Whether the subsystem is healthy")
	Attribute("message", String, "Human-readable status detail")
	Required("ok")
})

// ---------------------------------------------------------------------------
// The following endpoints are implemented as custom HTTP handlers, not
// through Goa codegen, because they serve non-JSON content:
//
// - GET /v1/media/{sample_id}        — raw image with Content-Type
// - GET /v1/media/{sample_id}/thumb  — JPEG thumbnail
// - GET /api/sse                     — SSE text/event-stream
// - GET /docs                        — Swagger UI HTML
// - GET /docs/openapi3.json          — OpenAPI 3 spec
//
// See: api/media.go, api/sse.go, api/docs.go
// ---------------------------------------------------------------------------
