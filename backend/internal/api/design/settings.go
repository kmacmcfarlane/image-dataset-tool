package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = Service("settings", func() {
	Description("Settings service for secrets management and configuration display")

	Method("get_info", func() {
		Description("Get system info: data dir path, encryption key status, and config values")
		Result(SettingsInfo)
		HTTP(func() {
			GET("/v1/settings/info")
			Response(StatusOK)
		})
	})

	Method("list_secrets", func() {
		Description("List secret keys (not values)")
		Result(SecretListResult)
		HTTP(func() {
			GET("/v1/settings/secrets")
			Response(StatusOK)
		})
	})

	Method("set_secret", func() {
		Description("Create or update a secret (value is encrypted on save)")
		Payload(SetSecretPayload)
		HTTP(func() {
			PUT("/v1/settings/secrets/{key}")
			Body(func() {
				Attribute("value")
			})
			Response(StatusNoContent)
		})
	})

	Method("delete_secret", func() {
		Description("Delete a secret by key")
		Payload(DeleteSecretPayload)
		HTTP(func() {
			DELETE("/v1/settings/secrets/{key}")
			Response(StatusNoContent)
		})
	})

	Method("test_provider", func() {
		Description("Test connection to an AI provider using stored credentials")
		Payload(TestProviderPayload)
		Result(TestProviderResult)
		HTTP(func() {
			POST("/v1/settings/providers/{provider}/test")
			Response(StatusOK)
		})
	})
})

var SettingsInfo = Type("SettingsInfo", func() {
	Description("System settings information")
	Attribute("data_dir", String, "Resolved data directory path")
	Attribute("key_status", String, "Encryption key status: found, missing, wrong_permissions")
	Attribute("config", MapOf(String, Any), "Configuration values grouped by section")
	Required("data_dir", "key_status", "config")
})

var SecretEntry = Type("SecretEntry", func() {
	Description("A secret entry (key only, no value)")
	Attribute("key", String, "Secret key name")
	Attribute("created_at", String, "Creation timestamp (RFC3339)")
	Attribute("updated_at", String, "Last update timestamp (RFC3339)")
	Required("key", "created_at", "updated_at")
})

var SecretListResult = Type("SecretListResult", func() {
	Description("List of secret keys")
	Attribute("secrets", ArrayOf(SecretEntry), "Secret entries")
	Required("secrets")
})

var SetSecretPayload = Type("SetSecretPayload", func() {
	Description("Payload for creating or updating a secret")
	Attribute("key", String, "Secret key name", func() {
		Pattern(`^[a-zA-Z0-9_\-\.]+$`)
		MaxLength(128)
	})
	Attribute("value", String, "Secret plaintext value (will be encrypted)")
	Required("key", "value")
})

var DeleteSecretPayload = Type("DeleteSecretPayload", func() {
	Description("Payload for deleting a secret")
	Attribute("key", String, "Secret key name")
	Required("key")
})

var TestProviderPayload = Type("TestProviderPayload", func() {
	Description("Payload for testing a provider connection")
	Attribute("provider", String, "Provider name (e.g. anthropic, openai)")
	Required("provider")
})

var TestProviderResult = Type("TestProviderResult", func() {
	Description("Provider connection test result")
	Attribute("success", Boolean, "Whether the connection test succeeded")
	Attribute("message", String, "Human-readable result message")
	Required("success", "message")
})
