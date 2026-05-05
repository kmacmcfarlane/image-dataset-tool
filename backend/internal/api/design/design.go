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

var _ = Service("health", func() {
	Description("Health check service")

	Method("check", func() {
		Description("Check system health")
		Result(HealthResult)
		HTTP(func() {
			GET("/health")
			Response(StatusOK)
		})
	})
})

var HealthResult = Type("HealthResult", func() {
	Attribute("status", String, "Overall health status", func() {
		Example("ok")
	})
	Required("status")
})
