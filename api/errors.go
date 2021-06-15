package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var (
	// ErrFuryServer is the error for 5xx server errors
	ErrFuryServer = errors.New("Something went wrong. Please contact support.")

	// ErrUnauthorized is the error for 401 from server
	ErrUnauthorized = errors.New("Authentication failure")

	// ErrForbidden is the error for 403 from server
	ErrForbidden = errors.New("You're not allowed to do this")

	// ErrNotFound is the error for 404 from server
	ErrNotFound = errors.New("Doesn't look like this exists")

	// Account has an exclusive locked for another operation
	ErrConflict = errors.New("Locked for another user. Try again later.")
)

// errorResponse is the JSON response for error from Gemfury API
type errorResponse struct {
	Error UserError
}

// Decode status to appropriate error from JSON error or HTTP code
func DecodeResponseError(resp *http.Response) error {
	if s := resp.StatusCode; s >= 200 && s <= 299 {
		return nil
	}

	apiErr := errorResponse{}
	err := json.NewDecoder(resp.Body).Decode(&apiErr)
	if err != nil || apiErr.Error.Type == "" {
		return StatusCodeToError(resp.StatusCode)
	}

	return apiErr.Error
}

// StatusCodeToError converts API response status to error code
func StatusCodeToError(s int) error {
	switch {
	case s == 401:
		return ErrUnauthorized
	case s == 403:
		return ErrForbidden
	case s == 404:
		return ErrNotFound
	case s == 409:
		return ErrConflict
	case s >= 200 && s < 300:
		return nil
	case s >= 500:
		return ErrFuryServer
	default:
		return fmt.Errorf(http.StatusText(s))
	}
}

// UserError is one whose message can be displayed to user
type UserError struct {
	Message string
	Type    string
}

// Error is a user-friendly error directly from the API's message
func (ue UserError) Error() string {
	return ue.Message
}

// StatusString is a shortened explanation for upload status (see "push")
func (ue UserError) ShortError() string {
	switch ue.Type {
	case "Conflict", "DupeVersion":
		return "this version already exists"
	case "GemVersionError", "InvalidGemFile":
		return "corrupt package file"
	case "Forbidden":
		return "no permission"
	default:
		return ue.Message
	}
}
