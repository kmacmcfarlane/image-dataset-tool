package model

import "time"

// IGRequestLog is a record of an IG API request for observability.
type IGRequestLog struct {
	ID             int64
	Timestamp      time.Time
	AccountID      string
	Endpoint       string
	HTTPStatus     int
	IsThrottled    bool
	IsDenied       bool
	ResponseTimeMs int64
	PublicIP       string
	ErrorDetail    string
}
