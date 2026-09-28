package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	// ErrFuryServer is the error for 5xx from server, other than 501
	ErrFuryServer = errors.New("Something went wrong. Please contact support.")

	// ErrTimeout is the error for 408 from server or net timeout
	ErrTimeout = errors.New("Operation timed out. Try again later.")

	// ErrUnauthorized is the error for 401 from server
	ErrUnauthorized = errors.New("Authentication failure")

	// ErrForbidden is the error for 403 from server
	ErrForbidden = errors.New("You're not allowed to do this")

	// ErrNotFound is the error for 404 from server
	ErrNotFound = errors.New("Doesn't look like this exists")

	// ErrTooManyRequests is the error for 429 from server
	ErrTooManyRequests = errors.New("Too many requests. Try again later.")

	// ErrConflict is the error for 409 from server: locked by another
	// operation, unless it is a version that exists (ErrAlreadyExists)
	ErrConflict = errors.New("Locked for update by another operation. Try again later.")

	// ErrAlreadyExists is the error for a version that already exists,
	// by the type of the error from server, whatever the status
	ErrAlreadyExists = errors.New("This version already exists")

	// ErrNotImplemented is the error for 501 from server
	ErrNotImplemented = errors.New("This operation is not supported")
)

// errorResponse is the JSON response for error from Gemfury API: an object
// with the error, or a list of those, as an upload has one for each file
type errorResponse struct {
	Error UserError
}

// UnmarshalJSON takes the first error of a list, or that of an object
func (er *errorResponse) UnmarshalJSON(data []byte) error {
	type object errorResponse // Without UnmarshalJSON, to not recurse
	if !bytes.HasPrefix(data, []byte("[")) {
		return json.Unmarshal(data, (*object)(er))
	}

	list := []object{}
	err := json.Unmarshal(data, &list)
	for _, item := range list {
		if item.Error != (UserError{}) {
			*er = errorResponse(item)
			break
		}
	}
	return err
}

// DecodeResponseError is the error of a failed response, with the message
// of the server, or else the one for its kind, as when the body is not JSON
func DecodeResponseError(resp *http.Response) error {
	if s := resp.StatusCode; s >= 200 && s <= 299 {
		return nil
	}

	apiErr := errorResponse{}
	body := io.LimitReader(resp.Body, maxErrorBytes)
	json.NewDecoder(body).Decode(&apiErr) // Best effort

	ue := apiErr.Error
	ue.Status = resp.StatusCode
	ue.RequestID = resp.Header.Get(hdrRequestID)
	if ue.Message == "" {
		ue.Message = ue.kind().Error()
	}

	return ue
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
	case s == 408:
		return ErrTimeout
	case s == 409:
		return ErrConflict
	case s == 429:
		return ErrTooManyRequests
	case s == 501:
		return ErrNotImplemented
	case s >= 200 && s < 300:
		return nil
	case s >= 500:
		return ErrFuryServer
	default:
		if text := http.StatusText(s); text != "" {
			return errors.New(text)
		}
		return fmt.Errorf("HTTP %d", s)
	}
}

// UserError is the error of a failed API response. Its message is
// that of the server, or else of its kind, and can be displayed.
type UserError struct {
	Message   string
	Type      string
	Status    int    `json:"-"` // Of the HTTP response
	RequestID string `json:"-"` // Of the HTTP response
}

// UnmarshalJSON takes an object with a message and a type, or a string
// that is the message
func (ue *UserError) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(data, []byte(`"`)) {
		return json.Unmarshal(data, &ue.Message)
	}

	type object UserError // Without UnmarshalJSON, to not recurse
	return json.Unmarshal(data, (*object)(ue))
}

// Error is the message of the API. That of a failing server
// has the status and request ID to report it to support by.
func (ue UserError) Error() string {
	switch {
	case !ue.Is(ErrFuryServer):
		return ue.Message
	case ue.RequestID == "":
		return fmt.Sprintf("%s (HTTP %d)", ue.Message, ue.Status)
	}
	return fmt.Sprintf("%s (HTTP %d, request ID %s)", ue.Message, ue.Status, ue.RequestID)
}

// Is lets errors.Is tell the kind of response, whatever its message
func (ue UserError) Is(target error) bool {
	return target == ue.kind()
}

// kind is the error for the type that the API gave
// to this response, or else for its status
func (ue UserError) kind() error {
	switch ue.Type {
	case "Unauthorized":
		return ErrUnauthorized
	case "Forbidden":
		return ErrForbidden
	case "Conflict", "DupeVersion":
		return ErrAlreadyExists
	}
	return StatusCodeToError(ue.Status)
}

// ShortError is a shortened explanation for upload status (see "push")
func (ue UserError) ShortError() string {
	switch ue.Type {
	case "GemVersionError", "InvalidGemFile":
		return "corrupt package file"
	default:
		return ue.Error()
	}
}
