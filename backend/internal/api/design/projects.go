package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = Service("projects", func() {
	Description("Project CRUD service")

	Method("list", func() {
		Description("List all projects")
		Result(ProjectListResult)
		HTTP(func() {
			GET("/v1/projects")
			Response(StatusOK)
		})
	})

	Method("get", func() {
		Description("Get a project by ID, including subject count")
		Payload(func() {
			Attribute("id", String, "Project ID (UUID)")
			Required("id")
		})
		Result(ProjectDetail)
		Error("not_found", ErrorResult, "Project not found")
		HTTP(func() {
			GET("/v1/projects/{id}")
			Response(StatusOK)
			Response("not_found", StatusNotFound)
		})
	})

	Method("create", func() {
		Description("Create a new project")
		Payload(CreateProjectPayload)
		Result(ProjectResult)
		Error("conflict", ErrorResult, "Duplicate slug")
		Error("bad_request", ErrorResult, "Invalid input")
		HTTP(func() {
			POST("/v1/projects")
			Body(func() {
				Attribute("name")
				Attribute("description")
			})
			Response(StatusCreated)
			Response("conflict", StatusConflict)
			Response("bad_request", StatusBadRequest)
		})
	})

	Method("update", func() {
		Description("Update project name and/or description")
		Payload(UpdateProjectPayload)
		Result(ProjectResult)
		Error("not_found", ErrorResult, "Project not found")
		Error("bad_request", ErrorResult, "Invalid input")
		HTTP(func() {
			PUT("/v1/projects/{id}")
			Body(func() {
				Attribute("name")
				Attribute("description")
			})
			Response(StatusOK)
			Response("not_found", StatusNotFound)
			Response("bad_request", StatusBadRequest)
		})
	})

	Method("delete", func() {
		Description("Delete a project (soft by default, hard with hard_delete=true)")
		Payload(func() {
			Attribute("id", String, "Project ID (UUID)")
			Attribute("hard_delete", Boolean, "Also remove project directory from disk", func() {
				Default(false)
			})
			Required("id")
		})
		Error("not_found", ErrorResult, "Project not found")
		HTTP(func() {
			DELETE("/v1/projects/{id}")
			Params(func() {
				Param("hard_delete")
			})
			Response(StatusNoContent)
			Response("not_found", StatusNotFound)
		})
	})
})

var ProjectResult = Type("ProjectResult", func() {
	Description("A project")
	Attribute("id", String, "Project UUID")
	Attribute("slug", String, "URL-friendly slug (auto-generated, immutable)")
	Attribute("name", String, "Project display name")
	Attribute("description", String, "Project description")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Attribute("updated_at", String, "Last update timestamp (RFC3339)")
	Required("id", "slug", "name", "description", "created_at", "updated_at")
})

var ProjectDetail = Type("ProjectDetail", func() {
	Description("A project with subject count")
	Attribute("id", String, "Project UUID")
	Attribute("slug", String, "URL-friendly slug")
	Attribute("name", String, "Project display name")
	Attribute("description", String, "Project description")
	Attribute("subject_count", Int, "Number of subjects in this project")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Attribute("updated_at", String, "Last update timestamp (RFC3339)")
	Required("id", "slug", "name", "description", "subject_count", "created_at", "updated_at")
})

var ProjectListResult = Type("ProjectListResult", func() {
	Description("List of projects")
	Attribute("projects", ArrayOf(ProjectResult), "Projects")
	Required("projects")
})

var CreateProjectPayload = Type("CreateProjectPayload", func() {
	Description("Payload for creating a project")
	Attribute("name", String, "Project display name", func() {
		MinLength(1)
		MaxLength(200)
	})
	Attribute("description", String, "Project description", func() {
		Default("")
		MaxLength(2000)
	})
	Required("name")
})

var UpdateProjectPayload = Type("UpdateProjectPayload", func() {
	Description("Payload for updating a project")
	Attribute("id", String, "Project ID (UUID)")
	Attribute("name", String, "Project display name", func() {
		MinLength(1)
		MaxLength(200)
	})
	Attribute("description", String, "Project description", func() {
		MaxLength(2000)
	})
	Required("id")
})
