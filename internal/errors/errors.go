package errors

import "fmt"

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Hint      string `json:"hint,omitempty"`
	Retryable bool   `json:"retryable"`
	ExitCode  int    `json:"exit_code"`
}

func (e *Error) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Hint)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) IsRetryable() bool {
	return e.Retryable
}

func (e *Error) CodeValue() string {
	return e.Code
}

func (e *Error) ExitCodeValue() int {
	return e.ExitCode
}

func Validation(code, message, hint string, retryable bool) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Hint:      hint,
		Retryable: retryable,
		ExitCode:  4,
	}
}

func Safety(message, hint string) *Error {
	return &Error{
		Code:      "SAFETY_COMMIT_REQUIRED",
		Message:   message,
		Hint:      hint,
		Retryable: false,
		ExitCode:  5,
	}
}

func External(code, message, hint string, retryable bool) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Hint:      hint,
		Retryable: retryable,
		ExitCode:  8,
	}
}
