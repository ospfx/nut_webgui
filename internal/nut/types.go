package nut

import (
	"errors"
	"fmt"
)

// UPSEntry represents a UPS device returned by LIST UPS.
type UPSEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// VarEntry represents a UPS variable.
type VarEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RangeEntry represents a min/max range constraint for a variable.
type RangeEntry struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

// NutError represents a NUT protocol error response.
type NutError struct {
	Code string
}

func (e *NutError) Error() string {
	return fmt.Sprintf("nut error: %s", e.Code)
}

// IsAccessDenied reports whether the error is an access-denied error.
func IsAccessDenied(err error) bool {
	var ne *NutError
	if errors.As(err, &ne) {
		return ne.Code == "ACCESS-DENIED"
	}
	return false
}

// IsUnknownUPS reports whether the error is an unknown UPS error.
func IsUnknownUPS(err error) bool {
	var ne *NutError
	if errors.As(err, &ne) {
		return ne.Code == "UNKNOWN-UPS"
	}
	return false
}

// IsVarNotSupported reports whether the error indicates an unsupported variable.
func IsVarNotSupported(err error) bool {
	var ne *NutError
	if errors.As(err, &ne) {
		return ne.Code == "VAR-NOT-SUPPORTED"
	}
	return false
}
