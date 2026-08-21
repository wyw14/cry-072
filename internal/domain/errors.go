package domain

type ProblemCode string

const (
	ProblemMissing          ProblemCode = "SAFETY_RESOURCE_MISSING"
	ProblemVersionConflict  ProblemCode = "SAFETY_VERSION_CONFLICT"
	ProblemRepeatedRequest  ProblemCode = "SAFETY_REPEATED_REQUEST"
	ProblemPermissionDenied ProblemCode = "SAFETY_PERMISSION_DENIED"
	ProblemStateBlocked     ProblemCode = "SAFETY_STATE_BLOCKED"
	ProblemInvalidInput     ProblemCode = "SAFETY_INVALID_INPUT"
	ProblemStoreUnavailable ProblemCode = "SAFETY_STORE_UNAVAILABLE"
)

type DomainProblem struct {
	Code    ProblemCode
	Summary string
}

func (p *DomainProblem) Error() string { return string(p.Code) + ": " + p.Summary }

var (
	ErrNotFound          = &DomainProblem{Code: ProblemMissing, Summary: "安全业务对象不存在"}
	ErrConflict          = &DomainProblem{Code: ProblemVersionConflict, Summary: "安全记录版本已变化"}
	ErrDuplicate         = &DomainProblem{Code: ProblemRepeatedRequest, Summary: "幂等请求已经处理"}
	ErrForbidden         = &DomainProblem{Code: ProblemPermissionDenied, Summary: "操作人没有所需权限"}
	ErrInvalidTransition = &DomainProblem{Code: ProblemStateBlocked, Summary: "隐患状态不允许该操作"}
	ErrValidation        = &DomainProblem{Code: ProblemInvalidInput, Summary: "输入不符合安全业务规则"}
	ErrNotReady          = &DomainProblem{Code: ProblemStoreUnavailable, Summary: "安全数据仓储尚未就绪"}
)

type FieldViolation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Violations []FieldViolation `json:"violations"`
}

func (e ValidationError) Error() string {
	if len(e.Violations) == 0 {
		return ErrValidation.Error()
	}
	return ErrValidation.Error() + ": " + e.Violations[0].Field + " " + e.Violations[0].Message
}

func (e ValidationError) Unwrap() error { return ErrValidation }

func NewValidationError(field, message string) error {
	return ValidationError{Violations: []FieldViolation{{Field: field, Message: message}}}
}
