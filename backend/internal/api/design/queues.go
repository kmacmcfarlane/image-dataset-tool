package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = Service("queues", func() {
	Description("Queue administration service for NATS JetStream pipeline")

	Method("stats", func() {
		Description("Get per-consumer queue statistics")
		Result(QueueStatsResult)
		HTTP(func() {
			GET("/v1/queues/stats")
			Response(StatusOK)
		})
	})

	Method("peek", func() {
		Description("Peek at messages in a queue without consuming them")
		Payload(PeekPayload)
		Result(PeekResult)
		HTTP(func() {
			GET("/v1/queues/{subject}/messages")
			Params(func() {
				Param("offset", Int, "Pagination offset", func() {
					Default(0)
					Minimum(0)
				})
				Param("limit", Int, "Pagination limit", func() {
					Default(20)
					Minimum(1)
					Maximum(100)
				})
			})
			Response(StatusOK)
		})
	})

	Method("retry", func() {
		Description("Redeliver a specific message to its original subject")
		Payload(RetryPayload)
		HTTP(func() {
			POST("/v1/queues/{subject}/retry")
			Response(StatusNoContent)
		})
	})

	Method("delete_message", func() {
		Description("Delete a specific message from a queue")
		Payload(DeleteMessagePayload)
		HTTP(func() {
			DELETE("/v1/queues/{subject}/messages/{sequence}")
			Response(StatusNoContent)
		})
	})

	Method("purge", func() {
		Description("Purge all messages from a queue subject")
		Payload(PurgePayload)
		HTTP(func() {
			POST("/v1/queues/{subject}/purge")
			Response(StatusNoContent)
		})
	})
})

var QueueStatsResult = Type("QueueStatsResult", func() {
	Description("Per-consumer queue statistics")
	Attribute("consumers", ArrayOf(ConsumerStats), "List of consumer statistics")
	Required("consumers")
})

var ConsumerStats = Type("ConsumerStats", func() {
	Attribute("name", String, "Consumer name")
	Attribute("subject", String, "Filter subject")
	Attribute("pending", Int64, "Number of pending messages")
	Attribute("ack_pending", Int64, "Number of messages awaiting acknowledgement")
	Attribute("redelivered", Int64, "Number of redelivered messages")
	Attribute("waiting", Int64, "Number of waiting pull requests")
	Required("name", "subject", "pending", "ack_pending", "redelivered", "waiting")
})

var PeekPayload = Type("PeekPayload", func() {
	Attribute("subject", String, "Queue subject to peek", func() {
		Example("media.dlq")
	})
	Attribute("offset", Int, "Pagination offset")
	Attribute("limit", Int, "Pagination limit")
	Required("subject")
})

var PeekResult = Type("PeekResult", func() {
	Attribute("messages", ArrayOf(QueueMessage), "Peeked messages")
	Attribute("total", Int64, "Total messages in subject")
	Required("messages", "total")
})

var QueueMessage = Type("QueueMessage", func() {
	Attribute("sequence", UInt64, "Stream sequence number")
	Attribute("subject", String, "Message subject")
	Attribute("data", String, "Message payload (base64 or UTF-8)")
	Attribute("headers", MapOf(String, String), "Message headers")
	Attribute("timestamp", String, "Message timestamp (RFC3339)")
	Required("sequence", "subject", "data", "timestamp")
})

var RetryPayload = Type("RetryPayload", func() {
	Attribute("subject", String, "Queue subject")
	Attribute("sequence", UInt64, "Sequence number of message to retry")
	Required("subject", "sequence")
})

var DeleteMessagePayload = Type("DeleteMessagePayload", func() {
	Attribute("subject", String, "Queue subject")
	Attribute("sequence", UInt64, "Sequence number of message to delete")
	Required("subject", "sequence")
})

var PurgePayload = Type("PurgePayload", func() {
	Attribute("subject", String, "Queue subject to purge")
	Required("subject")
})
