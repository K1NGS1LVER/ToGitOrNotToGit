package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrorKind classifies an LLM failure so callers can react to it - most
// importantly, cascading to another model only when a model is truly
// unavailable, rather than after an auth or timeout failure that another model
// cannot fix.
type ErrorKind int

const (
	KindUnknown ErrorKind = iota
	KindModelUnavailable
	KindAuth
	KindRateLimit
	KindServer
	KindTimeout
	KindNetwork
	KindInvalidOutput
)

func (k ErrorKind) String() string {
	switch k {
	case KindModelUnavailable:
		return "model-unavailable"
	case KindAuth:
		return "auth"
	case KindRateLimit:
		return "rate-limit"
	case KindServer:
		return "server"
	case KindTimeout:
		return "timeout"
	case KindNetwork:
		return "network"
	case KindInvalidOutput:
		return "invalid-output"
	default:
		return "unknown"
	}
}

// Error is a categorized LLM failure. Provider is the configured provider name,
// Model the model that failed, and Message the trimmed provider-supplied text.
type Error struct {
	Kind     ErrorKind
	Status   int
	Provider string
	Model    string
	Message  string
	Err      error
}

func (e *Error) Error() string {
	s := "llm: " + e.Kind.String()
	if e.Model != "" {
		s += " model=" + e.Model
	}
	if e.Provider != "" {
		s += " provider=" + e.Provider
	}
	if e.Status != 0 {
		s += fmt.Sprintf(" status=%d", e.Status)
	}
	if detail := e.detail(); detail != "" {
		s += ": " + detail
	}
	return s
}

// detail combines the provider-supplied message with the wrapped cause so callers
// (and logs) never lose the underlying reason.
func (e *Error) detail() string {
	switch {
	case e.Message != "" && e.Err != nil:
		return e.Message + ": " + e.Err.Error()
	case e.Message != "":
		return e.Message
	case e.Err != nil:
		return e.Err.Error()
	default:
		return ""
	}
}

func (e *Error) Unwrap() error { return e.Err }

// KindOf reports the ErrorKind of err, or KindUnknown when err is nil or is not
// an *Error.
func KindOf(err error) ErrorKind {
	if err == nil {
		return KindUnknown
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindUnknown
}

// IsModelUnavailable reports whether err means the requested model does not
// exist or is no longer served. This is the only failure worth retrying with a
// different model.
func IsModelUnavailable(err error) bool {
	return KindOf(err) == KindModelUnavailable
}

// providerError mirrors the {"error":{"message","type","code"}} body that
// OpenAI-compatible providers return on failure.
type providerError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// parseProviderError extracts the provider's error code and message, falling
// back to the raw body when it is not the expected JSON shape.
func parseProviderError(body []byte) (code, message string) {
	var pe providerError
	if err := json.Unmarshal(body, &pe); err == nil && (pe.Error.Message != "" || pe.Error.Code != "") {
		return pe.Error.Code, strings.TrimSpace(pe.Error.Message)
	}
	return "", strings.TrimSpace(string(body))
}

// classify maps an HTTP status and provider error code/message to an ErrorKind.
func classify(status int, code, message string) ErrorKind {
	switch code {
	case "model_not_found", "model_decommissioned":
		return KindModelUnavailable
	}
	if strings.Contains(strings.ToLower(message), "does not exist") {
		return KindModelUnavailable
	}
	switch {
	case status == 401 || status == 403:
		return KindAuth
	case status == 404:
		return KindModelUnavailable
	case status == 429:
		return KindRateLimit
	case status >= 500:
		return KindServer
	default:
		return KindUnknown
	}
}
