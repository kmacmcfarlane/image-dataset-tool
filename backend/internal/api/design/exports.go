package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = Service("exports", func() {
	Description("Export service — trigger server-side exports and download zip archives")

	Method("create", func() {
		Description("Trigger a server-side export via NATS pipeline")
		Payload(CreateExportPayload)
		Result(CreateExportResult)
		Error("bad_request", ErrorResult, "Invalid input or no kept samples")
		Error("not_found", ErrorResult, "Subject or study not found")
		HTTP(func() {
			POST("/v1/exports")
			Response(StatusAccepted)
			Response("bad_request", StatusBadRequest)
			Response("not_found", StatusNotFound)
		})
	})

	Method("download_zip", func() {
		Description("Stream a zip archive of exported samples for the given subjects and study")
		Payload(DownloadZipPayload)
		// SkipResponseBodyEncodeDecode gives us raw access to http.ResponseWriter
		// for streaming the zip archive on-the-fly.
		Result(func() {
			Attribute("content_type", String)
			Attribute("content_disposition", String)
			Required("content_type", "content_disposition")
		})
		Error("bad_request", ErrorResult, "Invalid input or no kept samples")
		Error("not_found", ErrorResult, "Subject or study not found")
		HTTP(func() {
			POST("/v1/exports/zip")
			Response(func() {
				Header("content_type:Content-Type")
				Header("content_disposition:Content-Disposition")
			})
			SkipResponseBodyEncodeDecode()
			Response("bad_request", StatusBadRequest)
			Response("not_found", StatusNotFound)
		})
	})
})

var CreateExportPayload = Type("CreateExportPayload", func() {
	Description("Payload for triggering a server-side export")
	Attribute("subject_ids", ArrayOf(String), "Subject UUIDs to export")
	Attribute("study_slug", String, "Caption study slug (optional)")
	Attribute("format", String, "Export format (default: kohya-ss)", func() {
		Default("kohya-ss")
	})
	Attribute("destination", String, "Override output directory (optional)")
	Required("subject_ids")
})

var CreateExportResult = Type("CreateExportResult", func() {
	Description("Result of triggering an export")
	Attribute("job_id", String, "Job run UUID")
	Attribute("trace_id", String, "Trace ID for pipeline tracking")
	Attribute("status", String, "Job status")
	Attribute("total_items", Int, "Total samples to export")
	Attribute("output_path", String, "Export output directory path")
	Required("job_id", "trace_id", "status", "total_items", "output_path")
})

var DownloadZipPayload = Type("DownloadZipPayload", func() {
	Description("Payload for streaming a zip archive download")
	Attribute("subject_ids", ArrayOf(String), "Subject UUIDs to include in zip")
	Attribute("study_slug", String, "Caption study slug (optional)")
	Attribute("format", String, "Export format (default: kohya-ss)", func() {
		Default("kohya-ss")
	})
	Required("subject_ids")
})
