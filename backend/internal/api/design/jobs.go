package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = Service("jobs", func() {
	Description("Job history service — paginated job run list with per-item message log")

	Method("list", func() {
		Description("List job runs with optional type/status filters")
		Payload(JobListPayload)
		Result(JobListResult)
		HTTP(func() {
			GET("/v1/jobs")
			Params(func() {
				Param("type")
				Param("status")
				Param("offset")
				Param("limit")
			})
			Response(StatusOK)
		})
	})

	Method("get", func() {
		Description("Get a job run detail with timing, outcome, and item counts")
		Payload(func() {
			Attribute("id", String, "Job run ID (UUID)")
			Required("id")
		})
		Result(JobDetail)
		Error("not_found", ErrorResult, "Job not found")
		HTTP(func() {
			GET("/v1/jobs/{id}")
			Response(StatusOK)
			Response("not_found", StatusNotFound)
		})
	})

	Method("list_messages", func() {
		Description("List messages for a job run; default level filter is warning")
		Payload(JobMessageListPayload)
		Result(JobMessageListResult)
		Error("not_found", ErrorResult, "Job not found")
		HTTP(func() {
			GET("/v1/jobs/{id}/messages")
			Params(func() {
				Param("level")
				Param("offset")
				Param("limit")
			})
			Response(StatusOK)
			Response("not_found", StatusNotFound)
		})
	})
})

var JobListPayload = Type("JobListPayload", func() {
	Attribute("type", String, "Filter by job type (e.g. sample_collection, captioning)")
	Attribute("status", String, "Filter by status (running, succeeded, failed, cancelled)")
	Attribute("offset", Int, "Pagination offset", func() {
		Default(0)
		Minimum(0)
	})
	Attribute("limit", Int, "Pagination limit", func() {
		Default(20)
		Minimum(1)
		Maximum(100)
	})
})

var JobResult = Type("JobResult", func() {
	Description("A job run summary")
	Attribute("id", String, "Job run UUID")
	Attribute("type", String, "Job type")
	Attribute("subject_id", String, "Subject UUID (optional)")
	Attribute("study_id", String, "Caption study UUID (optional)")
	Attribute("status", String, "Job status")
	Attribute("total_items", Int, "Total items (null until known)")
	Attribute("completed_items", Int, "Completed items count")
	Attribute("failed_items", Int, "Failed items count")
	Attribute("started_at", String, "Start timestamp (RFC3339)")
	Attribute("finished_at", String, "Finish timestamp (RFC3339, null if running)")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Required("id", "type", "status", "completed_items", "failed_items", "started_at", "created_at")
})

var JobDetail = Type("JobDetail", func() {
	Description("Full job run detail")
	Attribute("id", String, "Job run UUID")
	Attribute("type", String, "Job type")
	Attribute("subject_id", String, "Subject UUID (optional)")
	Attribute("study_id", String, "Caption study UUID (optional)")
	Attribute("status", String, "Job status")
	Attribute("total_items", Int, "Total items (null until known)")
	Attribute("completed_items", Int, "Completed items count")
	Attribute("failed_items", Int, "Failed items count")
	Attribute("started_at", String, "Start timestamp (RFC3339)")
	Attribute("finished_at", String, "Finish timestamp (RFC3339, null if running)")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Required("id", "type", "status", "completed_items", "failed_items", "started_at", "created_at")
})

var JobListResult = Type("JobListResult", func() {
	Description("Paginated list of job runs")
	Attribute("jobs", ArrayOf(JobResult), "Job runs")
	Attribute("total", Int64, "Total matching jobs")
	Required("jobs", "total")
})

var JobMessageListPayload = Type("JobMessageListPayload", func() {
	Attribute("id", String, "Job run ID (UUID)")
	Attribute("level", String, "Filter by level: error, warning, info", func() {
		Default("warning")
		Enum("error", "warning", "info")
	})
	Attribute("offset", Int, "Pagination offset", func() {
		Default(0)
		Minimum(0)
	})
	Attribute("limit", Int, "Pagination limit", func() {
		Default(50)
		Minimum(1)
		Maximum(200)
	})
	Required("id")
})

var JobMessage = Type("JobMessage", func() {
	Description("A single job log message")
	Attribute("id", String, "Message UUID")
	Attribute("job_run_id", String, "Parent job run UUID")
	Attribute("level", String, "Log level: error, warning, info")
	Attribute("sample_ref", String, "Sample reference (optional)")
	Attribute("account_handle", String, "Account handle (optional)")
	Attribute("message", String, "Message text")
	Attribute("created_at", String, "Timestamp (RFC3339)")
	Required("id", "job_run_id", "level", "message", "created_at")
})

var JobMessageListResult = Type("JobMessageListResult", func() {
	Description("Paginated list of job messages")
	Attribute("messages", ArrayOf(JobMessage), "Job messages")
	Attribute("total", Int64, "Total matching messages")
	Required("messages", "total")
})
