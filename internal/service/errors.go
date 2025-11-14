package service

type ErrorCode string

const (
	ErrorCodeTeamExists   ErrorCode = "TEAM_EXISTS"
	ErrorCodePRExists     ErrorCode = "PR_EXISTS"
	ErrorCodePRMerged     ErrorCode = "PR_MERGED"
	ErrorCodeNotAssigned  ErrorCode = "NOT_ASSIGNED"
	ErrorCodeNoCandidate  ErrorCode = "NO_CANDIDATE"
	ErrorCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrorCodeUnspecified  ErrorCode = "UNSPECIFIED"
	ErrorCodeInvalidBody  ErrorCode = "INVALID_BODY"
	ErrorCodeUserInactive ErrorCode = "USER_INACTIVE"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func NewError(code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

func (e *Error) Error() string {
	return e.Message
}
