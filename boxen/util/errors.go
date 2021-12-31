package util

import "errors"

var ErrIgnoredOption = errors.New("ignoredOption")

var ErrInspectionError = errors.New("inspectionError")
var ErrValidationError = errors.New("validationError")
var ErrAllocationError = errors.New("allocationError")
var ErrProvisionError = errors.New("provisionError")

var ErrCommandError = errors.New("commandError")

var ErrInstanceError = errors.New("instanceError")
var ErrConsoleError = errors.New("consoleError")
