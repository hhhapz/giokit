package kittest

import (
	"os"
	"slices"
)

// TB is the subset of testing.TB the harness uses. Declaring it here keeps
// package testing out of binaries that use New for screenshots (-shot), and
// lets tests pass their own implementation.
type TB interface {
	Helper()
	Name() string
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Skip(args ...any)
	Cleanup(func())
}

// RequireGPU reports whether a missing headless GPU must fail tests instead
// of skipping them. Set KITTEST_REQUIRE_GPU=1 in CI, or a runner without EGL
// reports ok having run nothing.
func RequireGPU() bool {
	v := os.Getenv("KITTEST_REQUIRE_GPU")
	return v != "" && v != "0"
}

// NoGPU skips t, or fails it when RequireGPU is set. Harness constructors call
// it when New returns an error.
func NoGPU(t TB, err error) {
	t.Helper()
	if RequireGPU() {
		t.Fatal(err)
	}
	t.Skip(err)
}

// standalone is the TB of a harness made by New, which has no test to report
// to: report records problems for Problems, and fatal panics. Its methods do
// nothing, so harness methods can mark themselves as helpers in either mode.
type standalone struct{}

func (standalone) Helper()               {}
func (standalone) Name() string          { return "kittest" }
func (standalone) Error(...any)          {}
func (standalone) Errorf(string, ...any) {}
func (standalone) Fatal(...any)          {}
func (standalone) Fatalf(string, ...any) {}
func (standalone) Skip(...any)           {}
func (standalone) Cleanup(func())        {}

// attached reports whether the harness reports to a test (NewT, NewTAt).
func (h *Harness) attached() bool { return h.tb != standalone{} }

// report fails the test with p, or records it for Problems when no test is
// attached.
func (h *Harness) report(p Problem) {
	h.tb.Helper()
	if !h.attached() {
		h.problems = append(h.problems, p)
		return
	}
	h.tb.Error(p.String())
}

// Problems returns a copy of the findings recorded while no test is
// attached (a harness from New). A harness from NewT reports them to the
// test instead, so this is empty.
func (h *Harness) Problems() []Problem { return slices.Clone(h.problems) }

// fatal stops the test with msg, or panics when no test is attached: the
// harness cannot go on (a missing widget, a failed GPU readback).
func (h *Harness) fatal(msg string) {
	h.tb.Helper()
	if h.attached() {
		h.tb.Fatal(msg)
	}
	panic(msg) // a TB whose Fatal returns must not let the caller continue
}

// requireTest panics unless the harness reports to a test: Expect methods
// have nowhere else to send a failure.
func (h *Harness) requireTest(method string) {
	if !h.attached() {
		panic("kittest: " + method + " needs a harness from NewT")
	}
}
