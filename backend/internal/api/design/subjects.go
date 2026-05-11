package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = Service("subjects", func() {
	Description("Subject CRUD service (scoped to a project)")

	Method("list", func() {
		Description("List all subjects in a project with sample counts")
		Payload(func() {
			Attribute("project_id", String, "Project ID (UUID)")
			Required("project_id")
		})
		Result(SubjectListResult)
		Error("not_found", ErrorResult, "Project not found")
		HTTP(func() {
			GET("/v1/projects/{project_id}/subjects")
			Response(StatusOK)
			Response("not_found", StatusNotFound)
		})
	})

	Method("get", func() {
		Description("Get a subject by ID with linked accounts and sample stats")
		Payload(func() {
			Attribute("project_id", String, "Project ID (UUID)")
			Attribute("id", String, "Subject ID (UUID)")
			Required("project_id", "id")
		})
		Result(SubjectDetail)
		Error("not_found", ErrorResult, "Subject not found")
		HTTP(func() {
			GET("/v1/projects/{project_id}/subjects/{id}")
			Response(StatusOK)
			Response("not_found", StatusNotFound)
		})
	})

	Method("create", func() {
		Description("Create a new subject in a project")
		Payload(CreateSubjectPayload)
		Result(SubjectResult)
		Error("not_found", ErrorResult, "Project not found")
		Error("conflict", ErrorResult, "Duplicate slug within project")
		Error("bad_request", ErrorResult, "Invalid input")
		HTTP(func() {
			POST("/v1/projects/{project_id}/subjects")
			Body(func() {
				Attribute("name")
			})
			Response(StatusCreated)
			Response("not_found", StatusNotFound)
			Response("conflict", StatusConflict)
			Response("bad_request", StatusBadRequest)
		})
	})

	Method("update", func() {
		Description("Update subject name; renames directory if slug changes")
		Payload(UpdateSubjectPayload)
		Result(SubjectResult)
		Error("not_found", ErrorResult, "Subject not found")
		Error("conflict", ErrorResult, "Duplicate slug within project")
		Error("bad_request", ErrorResult, "Invalid input")
		HTTP(func() {
			PUT("/v1/projects/{project_id}/subjects/{id}")
			Body(func() {
				Attribute("name")
			})
			Response(StatusOK)
			Response("not_found", StatusNotFound)
			Response("conflict", StatusConflict)
			Response("bad_request", StatusBadRequest)
		})
	})

	Method("delete", func() {
		Description("Delete a subject (soft by default, hard with hard_delete=true)")
		Payload(func() {
			Attribute("project_id", String, "Project ID (UUID)")
			Attribute("id", String, "Subject ID (UUID)")
			Attribute("hard_delete", Boolean, "Also remove subject directory from disk", func() {
				Default(false)
			})
			Required("project_id", "id")
		})
		Error("not_found", ErrorResult, "Subject not found")
		HTTP(func() {
			DELETE("/v1/projects/{project_id}/subjects/{id}")
			Params(func() {
				Param("hard_delete")
			})
			Response(StatusNoContent)
			Response("not_found", StatusNotFound)
		})
	})
})

var SubjectResult = Type("SubjectResult", func() {
	Description("A subject")
	Attribute("id", String, "Subject UUID")
	Attribute("project_id", String, "Parent project UUID")
	Attribute("slug", String, "URL-friendly slug (auto-generated from name)")
	Attribute("name", String, "Subject display name")
	Attribute("sample_count", Int, "Total number of samples")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Attribute("updated_at", String, "Last update timestamp (RFC3339)")
	Required("id", "project_id", "slug", "name", "sample_count", "created_at", "updated_at")
})

var SubjectDetail = Type("SubjectDetail", func() {
	Description("A subject with linked accounts and sample stats")
	Attribute("id", String, "Subject UUID")
	Attribute("project_id", String, "Parent project UUID")
	Attribute("slug", String, "URL-friendly slug")
	Attribute("name", String, "Subject display name")
	Attribute("linked_accounts", ArrayOf(SubjectLinkedAccount), "Accounts linked to this subject")
	Attribute("sample_stats", SubjectSampleStats, "Sample counts by status")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Attribute("updated_at", String, "Last update timestamp (RFC3339)")
	Required("id", "project_id", "slug", "name", "linked_accounts", "sample_stats", "created_at", "updated_at")
})

var SubjectLinkedAccount = Type("SubjectLinkedAccount", func() {
	Attribute("id", String, "Account UUID")
	Attribute("platform", String, "Platform name (e.g. instagram)")
	Attribute("handle", String, "Account handle")
	Required("id", "platform", "handle")
})

var SubjectSampleStats = Type("SubjectSampleStats", func() {
	Attribute("pending", Int, "Samples with pending status")
	Attribute("kept", Int, "Samples with kept status")
	Attribute("rejected", Int, "Samples with rejected status")
	Attribute("total", Int, "Total sample count")
	Required("pending", "kept", "rejected", "total")
})

var SubjectListResult = Type("SubjectListResult", func() {
	Description("List of subjects")
	Attribute("subjects", ArrayOf(SubjectResult), "Subjects")
	Required("subjects")
})

var CreateSubjectPayload = Type("CreateSubjectPayload", func() {
	Description("Payload for creating a subject")
	Attribute("project_id", String, "Parent project ID (UUID)")
	Attribute("name", String, "Subject display name", func() {
		MinLength(1)
		MaxLength(200)
	})
	Required("project_id", "name")
})

var UpdateSubjectPayload = Type("UpdateSubjectPayload", func() {
	Description("Payload for updating a subject")
	Attribute("project_id", String, "Parent project ID (UUID)")
	Attribute("id", String, "Subject ID (UUID)")
	Attribute("name", String, "Subject display name", func() {
		MinLength(1)
		MaxLength(200)
	})
	Required("project_id", "id")
})
