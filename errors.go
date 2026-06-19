package main

import (
	"errors"
	"fmt"
)

//  ── File not ready ───────────────────────────────────────────────────────
type FileNotReadyError struct {
	file    string
	ogError error
}

func (e FileNotReadyError) Error() string {
	return fmt.Sprintf("File not ready: %s. %s", e.file, e.ogError.Error())
}

func NewFileNotReadyError(f string) *FileNotReadyError {
	return &FileNotReadyError{
		file:    f,
		ogError: errors.New("File is not ready"),
	}
}

//  ── Command execution ────────────────────────────────────────────────────
type CommandExecutionError struct {
	file    string
	command string
	ogError error
}

func (e CommandExecutionError) Error() string {
	return fmt.Sprintf("Error running command:\n\t%s\n\t%s\n\t%s", e.file, e.command, e.ogError.Error())
}

func NewCommandExecutionError(f string, c string, e error) *CommandExecutionError {
	return &CommandExecutionError{
		file:    f,
		command: c,
		ogError: e,
	}
}

//  ── ContextCancellationError ─────────────────────────────────────────────
type CommandCancellationError struct {
	ogError error
}

func (e CommandCancellationError) Error() string {
	return fmt.Sprintf("Command canceled: %s", e.ogError.Error())
}

func NewCommandCancellationError(e error) *CommandCancellationError {
	return &CommandCancellationError{
		ogError: e,
	}
}
