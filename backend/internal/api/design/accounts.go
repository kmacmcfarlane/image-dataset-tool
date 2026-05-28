package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = Service("accounts", func() {
	Description("Social media account CRUD service")

	Method("list", func() {
		Description("List all accounts; never returns cookies")
		Result(AccountListResult)
		HTTP(func() {
			GET("/v1/accounts")
			Response(StatusOK)
		})
	})

	Method("get", func() {
		Description("Get an account by ID with linked subjects; never returns cookies")
		Payload(func() {
			Attribute("id", String, "Account ID (UUID)")
			Required("id")
		})
		Result(AccountDetailResult)
		Error("not_found", ErrorResult, "Account not found")
		HTTP(func() {
			GET("/v1/accounts/{id}")
			Response(StatusOK)
			Response("not_found", StatusNotFound)
		})
	})

	Method("create", func() {
		Description("Create a new account")
		Payload(CreateAccountPayload)
		Result(AccountResult)
		Error("conflict", ErrorResult, "Duplicate (platform, handle)")
		Error("bad_request", ErrorResult, "Invalid input")
		HTTP(func() {
			POST("/v1/accounts")
			Body(func() {
				Attribute("platform")
				Attribute("handle")
			})
			Response(StatusCreated)
			Response("conflict", StatusConflict)
			Response("bad_request", StatusBadRequest)
		})
	})

	Method("update_cookies", func() {
		Description("Update cookies for an account; encrypts with AES-256-GCM")
		Payload(UpdateCookiesPayload)
		Error("not_found", ErrorResult, "Account not found")
		Error("bad_request", ErrorResult, "Invalid input")
		HTTP(func() {
			PUT("/v1/accounts/{id}/cookies")
			Body(func() {
				Attribute("cookies")
			})
			Response(StatusNoContent)
			Response("not_found", StatusNotFound)
			Response("bad_request", StatusBadRequest)
		})
	})

	Method("delete", func() {
		Description("Delete an account and all subject_accounts links")
		Payload(func() {
			Attribute("id", String, "Account ID (UUID)")
			Required("id")
		})
		Error("not_found", ErrorResult, "Account not found")
		HTTP(func() {
			DELETE("/v1/accounts/{id}")
			Response(StatusNoContent)
			Response("not_found", StatusNotFound)
		})
	})
})

var AccountResult = Type("AccountResult", func() {
	Description("An account (never includes cookies)")
	Attribute("id", String, "Account UUID")
	Attribute("platform", String, "Platform name (e.g. instagram)")
	Attribute("handle", String, "Account handle")
	Attribute("has_cookies", Boolean, "Whether cookies are stored")
	Attribute("last_login_at", String, "Last login timestamp (RFC3339), empty if never")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Attribute("updated_at", String, "Last update timestamp (RFC3339)")
	Required("id", "platform", "handle", "has_cookies", "last_login_at", "created_at", "updated_at")
})

var AccountDetailResult = Type("AccountDetailResult", func() {
	Description("An account with linked subjects (never includes cookies)")
	Attribute("id", String, "Account UUID")
	Attribute("platform", String, "Platform name")
	Attribute("handle", String, "Account handle")
	Attribute("has_cookies", Boolean, "Whether cookies are stored")
	Attribute("last_login_at", String, "Last login timestamp (RFC3339), empty if never")
	Attribute("linked_subjects", ArrayOf(AccountLinkedSubjectResult), "Subjects linked to this account")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Attribute("updated_at", String, "Last update timestamp (RFC3339)")
	Required("id", "platform", "handle", "has_cookies", "last_login_at", "linked_subjects", "created_at", "updated_at")
})

var AccountLinkedSubjectResult = Type("AccountLinkedSubjectResult", func() {
	Attribute("id", String, "Subject UUID")
	Attribute("project_id", String, "Project UUID")
	Attribute("name", String, "Subject display name")
	Required("id", "project_id", "name")
})

var AccountListResult = Type("AccountListResult", func() {
	Description("List of accounts")
	Attribute("accounts", ArrayOf(AccountResult), "Accounts")
	Required("accounts")
})

var CreateAccountPayload = Type("CreateAccountPayload", func() {
	Description("Payload for creating an account")
	Attribute("platform", String, "Platform name (e.g. instagram)", func() {
		MinLength(1)
		MaxLength(50)
	})
	Attribute("handle", String, "Account handle", func() {
		MinLength(1)
		MaxLength(200)
	})
	Required("platform", "handle")
})

var UpdateCookiesPayload = Type("UpdateCookiesPayload", func() {
	Description("Payload for updating account cookies")
	Attribute("id", String, "Account ID (UUID)")
	Attribute("cookies", String, "Cookie string to encrypt and store")
	Required("id", "cookies")
})
