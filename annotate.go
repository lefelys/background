package background

import (
	"context"
	"fmt"
)

type annotationBackground struct {
	*group

	annotation string
}

// WithAnnotation returns new Background with merged children and assigned annotation to it.
func WithAnnotation(message string, children ...Background) Background {
	return withAnnotation(message, children...)
}

func withAnnotation(message string, children ...Background) *annotationBackground {
	return &annotationBackground{
		group:      merge(children...),
		annotation: message,
	}
}

// Err returns the first encountered error in Background's children annotated
// with background's annotation.
// Returns nil if no errors found.
func (a *annotationBackground) Err() error {
	for _, m := range a.backgrounds {
		if err := m.Err(); err != nil {
			return fmt.Errorf("%s: %w", a.annotation, err)
		}
	}

	return nil
}

// Shutdown shuts down Background's children and returns annotated shutdown error.
// Returns nil no errors occurred.
func (a *annotationBackground) Shutdown(ctx context.Context) error {
	if err := a.group.Shutdown(ctx); err != nil {
		return fmt.Errorf("%s: %w", a.annotation, err)
	}

	return nil
}

func (a *annotationBackground) DependsOn(children ...Background) Background {
	return withDependency(a, children...)
}

func (a *annotationBackground) cause() error {
	if err := a.group.cause(); err != nil {
		return fmt.Errorf("%s: %w", a.annotation, err)
	}

	return nil
}
