package domain

import (
	"fmt"
	"strings"
)

const (
	blankFieldErrMsg = "can't be blank"
	DuplicateErrMsg  = "has already been taken"
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

type DuplicateError struct {
	Field string
	Msg   string
}

func (d *DuplicateError) Error() string {
	return fmt.Sprintf("field %s: %s", d.Field, d.Msg)
}

func NewDuplicateError(field string) *DuplicateError {
	return &DuplicateError{
		Field: field,
		Msg:   DuplicateErrMsg,
	}
}

type CredentialsError struct{}

func (c *CredentialsError) Error() string {
	return "invalid credentials"
}

type ProfileNotFoundError struct{}

func (p *ProfileNotFoundError) Error() string {
	return "profile not found"
}

type ArticleNotFoundError struct{}

func (a *ArticleNotFoundError) Error() string {
	return "article not found"
}
