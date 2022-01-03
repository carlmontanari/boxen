package util

import "errors"

// ErrIgnoredOption is an error returned when any option is not applicable for the given operation
// -- when this error is returned it is *usually* safe to ignore.
var ErrIgnoredOption = errors.New("ignoredOption")

// ErrInspectionError is an error returned when an issue "inspecting" an instance is encountered.
var ErrInspectionError = errors.New("inspectionError")

// ErrValidationError is an error returned when validating provided information, usually from a
// user, is encountered.
var ErrValidationError = errors.New("validationError")

// ErrAllocationError is an error returned when boxen encounters an issue allocating resources to a
// virtual machine.
var ErrAllocationError = errors.New("allocationError")

// ErrProvisionError is an error returned when provisioning a virtual machine in the local
// configuration fails.
var ErrProvisionError = errors.New("provisionError")

// ErrCommandError is an error returned when a command fails to execute.
var ErrCommandError = errors.New("commandError")

// ErrInstanceError is an error returned when a "well-formed"/created instance (as in, an instance
// that gets to the point of starting) encounters an error.
var ErrInstanceError = errors.New("instanceError")

// ErrConsoleError is an error returned when connecting to a device console produces an error, or
// when a console operation fails.
var ErrConsoleError = errors.New("consoleError")
