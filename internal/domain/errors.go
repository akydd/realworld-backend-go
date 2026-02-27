package domain

import (
	"fmt"
	"strings"
)

const (
	blankFieldErrMsg = "can't be blank"
)

type ValidationError struct {
	Field  string
	Errors []string
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("field %s, errors %s", v.Field, strings.Join(v.Errors, ","))
}

func NewValidationError(field string, err string) *ValidationError {
	return &ValidationError{
		Field:  field,
		Errors: []string{err},
	}
}
