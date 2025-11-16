package model

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
	ErrorCodeUnauthorized ErrorCode = "UNAUTHORIZED"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Cause   error     `json:"-"`
}

func NewError(code ErrorCode, message string, cause ...error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause: func() error {
			if len(cause) > 0 {
				return cause[0]
			} else {
				return nil
			}
		}(),
	}
}

func (e *Error) Error() string {
	return e.Message
}
