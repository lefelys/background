package background

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestParallel(t *testing.T) {
	t.Run("background", func(t *testing.T) {
		// Group
		t.Run("GroupClose", GroupCloseTest)
		t.Run("GroupSuccessiveClose", GroupSuccessiveCloseTest)
		t.Run("GroupError", GroupErrorTest)
		t.Run("GroupNilChild", GroupNilChildTest)

		// Shutdown
		t.Run("ShutdownWrap", ShutdownWrapTest)
		t.Run("ShutdownSuccessiveDone", ShutdownSuccessiveDoneTest)
		t.Run("ShutdownSuccessiveCall", ShutdownSuccessiveCallTest)
		t.Run("ShutdownTimeout", ShutdownTimeoutTest)
		t.Run("ShutdownUnclosed", ShutdownUnclosedTest)

		// Wait
		t.Run("Wait", WaitTest)

		// Readiness
		t.Run("ReadinessWrap", ReadinessWrapTest)
		t.Run("ReadinessSuccessiveOk", ReadinessSuccessiveOkTest)
		t.Run("ReadinessSuccessiveReady", ReadinessSuccessiveReadyTest)

		// Value
		t.Run("ValueWrap", ValueWrapTest)
		t.Run("ValueChildren", ValueChildrenTest)
		t.Run("ValueNilPanic", ValueNilPanicTest)
		t.Run("ValueComparablePanic", ValueComparablePanicTest)

		// Annotation
		t.Run("AnnotationError", AnnotationErrorTest)
		t.Run("AnnotationShutdownTimeout", AnnotationShutdownTimeoutTest)
		t.Run("AnnotationChildShutdownTimeout", AnnotationChildShutdownTimeoutTest)
		t.Run("AnnotationNilError", AnnotationNilErrorTest)
		t.Run("AnnotationNilShutdownError", AnnotationNilShutdownErrorTest)
		t.Run("AnnotationUnclosed", AnnotationUnclosedTest)

		// Error
		t.Run("Error", ErrorTest)

		// Error group
		t.Run("ErrorGroup", ErrorGroupTest)
		t.Run("ErrorGroupErrorf", ErrorGroupErrorfTest)

		// Empty
		t.Run("Empty", EmptyTest)

		// Dependency
		t.Run("DependencyShutdown", DependencyShutdownTest)
		t.Run("DependencyShutdownChain", DependencyShutdownChainTest)
		t.Run("DependencyShutdownSuccessiveClose", DependencyShutdownSuccessiveCloseTest)
		t.Run("DependencyShutdownChildrenTimeout", DependencyShutdownChildrenTimeoutTest)
		t.Run("DependencyShutdownParentTimeout", DependencyShutdownParentTimeoutTest)
		t.Run("DependencyShutdownUnclosed", DependencyShutdownUnclosedTest)
		t.Run("DependencyWait", DependencyWaitTest)
		t.Run("DependencyReadiness", DependencyReadinessTest)
		t.Run("DependencyErrorParent", DependencyErrorParentTest)
		t.Run("DependencyErrorChildren", DependencyErrorChildrenTest)
		t.Run("DependencyErrorNil", DependencyErrorNilTest)
		t.Run("DependencyValueParent", DependencyValueParentTest)
		t.Run("DependencyValueChildren", DependencyValueChildrenTest)
		t.Run("DependencyAnnotation", DependencyAnnotationTest)
	})
}

const (
	failTimeout = 100 * time.Millisecond

	errInitClosed    = "shutdown Background initialized closed"
	errNotClosed     = "shutdown Background didn't trigger close"
	errNotFinished   = "shutdown Background didn't finish"
	errClosed        = "unexpected close of shutdown Background"
	errFinished      = "unexpected finish of shutdown Background"
	errTimeout       = "unexpected timeout of shutdown Background"
	errNotWaited     = "wait Background didn't wait"
	errFinishWaiting = "wait Background didn't finish waiting"
	errInitReady     = "readiness Background initialized as ready"
	errNotReady      = "readiness Background isn't ready"
	errReady         = "unexpected readiness of readiness Background"
)

func runShutdownable(tail ShutdownTail) (okDone chan struct{}) {
	okDone = make(chan struct{})

	go func() {
		<-tail.End()
		<-okDone
		tail.Done()
	}()

	return
}

func runWaitable(tail WaitTail) (okWait chan struct{}) {
	okWait = make(chan struct{})

	tail.Add(1)

	go func() {
		<-okWait
		tail.Done()
	}()

	return
}

// checks any of passed channels are closed
func hasClosed(cc ...<-chan struct{}) bool {
	for _, c := range cc {
		select {
		case <-c:
			return true
		default:
		}
	}

	return false
}

// checks any of passed channels are not closed
func hasNotClosed(cc ...<-chan struct{}) bool {
	for _, c := range cc {
		select {
		case <-c:
		default:
			return true
		}
	}

	return false
}

func closeChanAndPropagate(cc ...chan struct{}) {
	for _, c := range cc {
		close(c)
	}

	time.Sleep(failTimeout)
}

// Group

func GroupCloseTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = withShutdown()

		okDone1 = runShutdownable(bg1)
		okDone2 = runShutdownable(bg2)

		bg3 = merge(bg1, bg2)
	)

	go bg3.close()
	closeChanAndPropagate(okDone1, okDone2)

	switch {
	case hasNotClosed(bg1.end, bg2.end, bg3.done):
		t.Error(errNotClosed)
	case hasNotClosed(bg1.done, bg2.done, bg3.finished):
		t.Error(errNotFinished)
	}
}

func GroupSuccessiveCloseTest(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive close call caused panic")
		}
	}()

	var (
		bg1 = withShutdown()

		okDone1 = runShutdownable(bg1)

		bg3 = merge(bg1)
	)

	closeChanAndPropagate(okDone1)
	bg3.close()
	bg3.close()
}

func GroupErrorTest(t *testing.T) {
	t.Parallel()

	var (
		err = errors.New("test")
		bg1 = withError(err)
		bg2 = emptyBackground{}
		bg3 = emptyBackground{}

		bg4 = merge(bg1, bg2, bg3)
		bg5 = merge(bg2, bg3)
	)

	if err := bg4.Err(); err == nil {
		t.Errorf("group Background with error Background didn't return error")
	}

	if err := bg5.Err(); err != nil {
		t.Errorf("group Background without error Background returned error")
	}
}

func GroupNilChildTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = emptyBackground{}
		bg2 = merge(bg1, nil)
	)

	if len(bg2.backgrounds) != 1 {
		t.Errorf("wrong number of group children: want 1, have %d", len(bg2.backgrounds))
		return
	}

	if bg2.backgrounds[0] == nil {
		t.Errorf("group has nil child")
	}
}

// Shutdown

func ShutdownWrapTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = withShutdown(bg1)
		bg3 = withShutdown(bg2)

		okDone1 = runShutdownable(bg1)
		okDone2 = runShutdownable(bg2)
		okDone3 = runShutdownable(bg3)
	)

	if hasClosed(
		bg1.end, bg2.end, bg3.end,
		bg1.done, bg2.done, bg3.done,
	) {
		t.Error(errInitClosed)
	}

	go bg3.close()
	time.Sleep(failTimeout)

	switch {
	case hasNotClosed(bg1.end):
		t.Error(errNotClosed)
	case hasClosed(bg2.end, bg3.end):
		t.Error(errClosed)
	case hasClosed(bg1.done, bg2.done, bg3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone1)

	switch {
	case hasNotClosed(bg2.end):
		t.Error(errNotClosed)
	case hasNotClosed(bg1.done):
		t.Error(errNotFinished)
	case hasClosed(bg3.end):
		t.Error(errClosed)
	case hasClosed(bg2.done, bg3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone2)

	switch {
	case hasNotClosed(bg2.done):
		t.Error(errNotFinished)
	case hasNotClosed(bg3.end):
		t.Error(errNotClosed)
	case hasClosed(bg3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone3)

	// bg3 must be done
	if hasNotClosed(bg3.done) {
		t.Error(errNotFinished)
	}
}

func ShutdownSuccessiveDoneTest(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive Done call caused panic")
		}
	}()

	var (
		bg1 = withShutdown()

		okDone1 = runShutdownable(bg1)
	)

	go bg1.close()

	closeChanAndPropagate(okDone1)
	bg1.Done()
}

func ShutdownSuccessiveCallTest(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive Shutdown call caused panic")
		}
	}()

	var (
		bg1 = withShutdown()

		okDone1 = runShutdownable(bg1)
	)

	go bg1.close()
	closeChanAndPropagate(okDone1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg1.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}

	err = bg1.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}
}

func ShutdownTimeoutTest(t *testing.T) {
	t.Parallel()

	bg := withShutdown()

	// blocked finish
	_ = runShutdownable(bg)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}
}

func ShutdownUnclosedTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = withShutdown(bg1)
	)

	okDone2 := runShutdownable(bg2)
	okDone1 := runShutdownable(bg1)

	closeChanAndPropagate(okDone1, okDone2)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg2.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}

	if err := bg1.cause(); err != nil {
		t.Errorf(errNotFinished)
	}

	if err := bg2.cause(); err != nil {
		t.Errorf(errNotFinished)
	}
}

// Wait

func WaitTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withWait()
		bg2 = withWait(bg1)
		bg3 = withWait(bg2)

		okDone1 = runWaitable(bg1)
		okDone2 = runWaitable(bg2)
		okDone3 = runWaitable(bg3)
	)

	done := make(chan struct{})

	go func() {
		bg3.Wait()
		close(done)
	}()

	time.Sleep(failTimeout)

	if hasClosed(done) {
		t.Error(errNotWaited)
	}

	closeChanAndPropagate(okDone1, okDone2)

	if hasClosed(done) {
		t.Error(errNotWaited)
	}

	closeChanAndPropagate(okDone3)

	time.Sleep(failTimeout)

	if hasNotClosed(done) {
		t.Error(errFinishWaiting)
	}
}

// Readiness

func ReadinessWrapTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withReadiness()
		bg2 = withReadiness(bg1)
		bg3 = withReadiness(bg2)
	)

	if hasClosed(
		bg1.ready, bg2.ready, bg3.ready,
	) {
		t.Error(errInitReady)
	}

	bg1ReadyC := bg1.Ready()
	bg2ReadyC := bg2.Ready()
	bg3ReadyC := bg3.Ready()

	bg3.Ok()

	time.Sleep(failTimeout)

	switch {
	case hasNotClosed(bg3.ready):
		t.Error(errNotReady)
	case hasClosed(bg1.ready, bg2.ready, bg1ReadyC, bg2ReadyC, bg3ReadyC):
		t.Error(errReady)
	}

	bg2.Ok()
	time.Sleep(failTimeout)

	switch {
	case hasNotClosed(bg2.ready, bg3.ready):
		t.Error(errNotReady)
	case hasClosed(bg1.ready, bg1ReadyC, bg2ReadyC, bg3ReadyC):
		t.Error(errReady)
	}

	bg1.Ok()
	time.Sleep(failTimeout)

	if hasNotClosed(bg1.ready, bg2.ready, bg3.ready, bg1ReadyC, bg2ReadyC, bg3ReadyC) {
		t.Error(errNotReady)
	}
}

func ReadinessSuccessiveOkTest(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive Ok call caused panic")
		}
	}()

	bg1 := withReadiness()

	bg1.Ok()
	bg1.Ok()
	bg1.Ok()
	readyC := bg1.Ready()

	time.Sleep(failTimeout)

	if hasNotClosed(readyC) {
		t.Error(errNotReady)
	}
}

func ReadinessSuccessiveReadyTest(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive Ready call caused panic")
		}
	}()

	bg1 := withReadiness()
	bg1.Ok()
	readyC := bg1.Ready()

	time.Sleep(failTimeout)

	if hasNotClosed(readyC) {
		t.Error(errNotReady)
	}

	readyC = bg1.Ready()

	if hasNotClosed(readyC) {
		t.Error(errNotReady)
	}
}

// Value

type key string

func ValueWrapTest(t *testing.T) {
	t.Parallel()

	var (
		testKey   = key("test_key")
		testValue = "test_value"
		bg1       = withValue(testKey, testValue)
		bg2       = withWait(bg1)
		bg3       = withWait(bg2)
	)

	value := bg3.Value(testKey)
	if value == nil {
		t.Error("test value for test key not found")
	}

	valueTyped, ok := value.(string)
	if !ok {
		t.Error("test value for test key has wrong type")
	}

	if valueTyped != testValue {
		t.Errorf("wrong test value: want %s have %s", testValue, valueTyped)
	}
}

func ValueChildrenTest(t *testing.T) {
	t.Parallel()

	var (
		testKey1   = key("test_key1")
		testValue1 = "test_value2"

		testKey2   = key("test_key2")
		testValue2 = "test_value2"

		testKeyNotFound = key("test_key3")
		bg1             = withValue(testKey1, testValue1)
		bg2             = withValue(testKey2, testValue2, bg1)
	)

	value := bg2.Value(testKey1)
	if value == nil {
		t.Error("test value 1 for test key not found")
	}

	valueTyped, ok := value.(string)
	if !ok {
		t.Error("test value 1 for test key has wrong type")
	}

	if valueTyped != testValue1 {
		t.Errorf("wrong test value 1: want %s have %s", testValue1, valueTyped)
	}

	value2 := bg2.Value(testKey2)
	if value2 == nil {
		t.Error("test value 2 for test key not found")
	}

	value2Typed, ok := value2.(string)
	if !ok {
		t.Error("test value 2 for test key has wrong type")
	}

	if value2Typed != testValue2 {
		t.Errorf("wrong test value 2: want %s have %s", testValue1, valueTyped)
	}

	value3 := bg2.Value(testKeyNotFound)
	if value3 != nil {
		t.Error("unused key returned non-nil value")
	}
}

func ValueNilPanicTest(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("nil key did not panic")
		}
	}()

	_ = withValue(nil, "")
}

func ValueComparablePanicTest(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("incomparable key did not panic")
		}
	}()

	_ = withValue(func() {}, "")
}

// Annotate

func AnnotationErrorTest(t *testing.T) {
	t.Parallel()

	const annotation = "test"

	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")

		bg1 = withError(err1)
		bg2 = withError(err2, bg1)
		bg3 = withAnnotation(annotation, bg2)
	)

	err := bg3.Err()

	// err2 is higher in the tree so it must be found first
	if !errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err2, err)
	}

	wantErrStr := fmt.Sprintf("%s: %s", annotation, err2.Error())

	if err.Error() != wantErrStr {
		t.Errorf("timeout error is not annotated, want error '%s', have '%s'", wantErrStr, err.Error())
	}

	err = bg1.Err()
	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}
}

func AnnotationShutdownTimeoutTest(t *testing.T) {
	t.Parallel()

	const annotation = "test"

	bg1 := withShutdown()
	bg2 := withAnnotation(annotation, bg1)

	// blocked finish
	_ = runShutdownable(bg1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg2.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}

	wantErrStr := fmt.Sprintf("%s: %s", annotation, ErrTimeout.Error())

	if err.Error() != wantErrStr {
		t.Errorf("timeout error is not annotated, want error '%s', have '%s'", wantErrStr, err.Error())
	}
}

func AnnotationChildShutdownTimeoutTest(t *testing.T) {
	t.Parallel()

	const annotation = "test"

	bg1 := withShutdown()
	bg2 := withAnnotation(annotation, bg1)
	bg3 := withShutdown(bg2)

	// blocked finish
	_ = runShutdownable(bg1)
	_ = runShutdownable(bg3)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg3.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}

	wantErrStr := fmt.Sprintf("%s: %s", annotation, ErrTimeout.Error())

	if err.Error() != wantErrStr {
		t.Errorf("timeout error is not annotated, want error '%s', have '%s'", wantErrStr, err.Error())
	}
}

func AnnotationNilErrorTest(t *testing.T) {
	t.Parallel()

	const message = "test"

	bg1 := withError(nil)
	bg2 := withAnnotation(message, bg1)

	if err := bg2.Err(); err != nil {
		t.Errorf("annotated Background returned error instead of nil")
	}
}

func AnnotationNilShutdownErrorTest(t *testing.T) {
	t.Parallel()

	bg1 := withShutdown()
	bg2 := withAnnotation("", bg1)

	// blocked finish
	okDone1 := runShutdownable(bg1)

	closeChanAndPropagate(okDone1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	if err := bg2.Shutdown(ctx); err != nil {
		t.Errorf("annotation Background returned error instead of nil")
	}
}

func AnnotationUnclosedTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = withAnnotation("", bg1)
	)

	okDone1 := runShutdownable(bg1)

	closeChanAndPropagate(okDone1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg2.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}

	if err := bg1.cause(); err != nil {
		t.Errorf(errNotFinished)
	}

	if err := bg2.cause(); err != nil {
		t.Errorf(errNotFinished)
	}
}

// Error

func ErrorTest(t *testing.T) {
	t.Parallel()

	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")

		bg1 = withError(err1)
		bg2 = withError(err2, bg1)
	)

	err := bg2.Err()

	// err2 is higher in the tree so it must be found first
	if !errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err2, err)
	}

	err = bg1.Err()
	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}
}

// Error group

func ErrorGroupTest(t *testing.T) {
	t.Parallel()

	const annotation = "test"

	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")
		bg1  = withErrorGroup()
		bg2  = withAnnotation(annotation, bg1)
	)

	if err := bg2.Err(); err != nil {
		t.Errorf("new error group Background returned error")
	}

	go func() {
		bg1.Error(err1)
	}()

	time.Sleep(failTimeout)

	err := bg2.Err()

	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}

	wantErrStr := fmt.Sprintf("%s: %s", annotation, err1.Error())

	if err.Error() != wantErrStr {
		t.Errorf("error group error is not annotated, want '%s', have '%s'", wantErrStr, err.Error())
	}

	go func() {
		bg1.Error(err2)
	}()

	time.Sleep(failTimeout)

	err = bg2.Err()

	if errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err2)
	}
}

func ErrorGroupErrorfTest(t *testing.T) {
	t.Parallel()

	const annotation = "test: %w"

	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")
		bg1  = withErrorGroup()
		bg2  = merge(bg1)
	)

	if err := bg2.Err(); err != nil {
		t.Errorf("new error group Background returned error")
	}

	go func() {
		bg1.Errorf(annotation, err1)
	}()

	time.Sleep(failTimeout)

	err := bg2.Err()

	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}

	wantErrStr := fmt.Sprintf("%s: %s", "test", err1.Error())

	if err.Error() != wantErrStr {
		t.Errorf("error group error is not annotated, want '%s', have '%s'", wantErrStr, err.Error())
	}

	go func() {
		bg1.Errorf(annotation, err2)
	}()

	time.Sleep(failTimeout)

	err = bg2.Err()

	if errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err2)
	}
}

// Empty

func EmptyTest(t *testing.T) {
	t.Parallel()

	bg1 := emptyBackground{}

	if err := bg1.Err(); err != nil {
		t.Errorf("empty Background returned error")
	}

	if err := bg1.Shutdown(context.Background()); err != nil {
		t.Errorf("empty Background shutdowned with error")
	}

	okDone1 := make(chan struct{})

	go func() {
		bg1.Wait()
		close(okDone1)
	}()

	time.Sleep(failTimeout)

	if hasNotClosed(okDone1) {
		t.Errorf("empty Background blocked on wait")
	}

	readyC := bg1.Ready()
	if hasNotClosed(readyC) {
		t.Errorf("empty Background is not ready")
	}

	if value := bg1.Value(""); value != nil {
		t.Errorf("empty Background returned value")
	}

	okDone2 := make(chan struct{})

	go func() {
		bg1.close()
		close(okDone2)
	}()

	time.Sleep(failTimeout)

	if hasNotClosed(okDone2) {
		t.Errorf("empty Background blocked on close")
	}

	if hasNotClosed(bg1.finishSig()) {
		t.Errorf("empty Background is not done")
	}

	if err := bg1.cause(); err != nil {
		t.Errorf("empty Background cause call returned error")
	}
}

// Dependency

func DependencyShutdownTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = withShutdown()
		bg3 = withShutdown()

		okDone1 = runShutdownable(bg1)
		okDone2 = runShutdownable(bg2)
		okDone3 = runShutdownable(bg3)
	)

	bg4 := withDependency(bg3, bg1, bg2)

	if hasClosed(
		bg1.end, bg2.end, bg3.end,
		bg1.done, bg2.done, bg3.done,
	) {
		t.Error(errInitClosed)
	}

	go bg4.close()
	time.Sleep(failTimeout)

	switch {
	case hasNotClosed(bg1.end, bg2.end):
		t.Error(errNotClosed)
	case hasClosed(bg3.end):
		t.Error(errClosed)
	case hasClosed(bg1.done, bg2.done, bg3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone1, okDone2)

	switch {
	case hasNotClosed(bg3.end):
		t.Error(errNotClosed)
	case hasNotClosed(bg1.done, bg2.done):
		t.Error(errNotFinished)
	case hasClosed(bg3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone3)

	if hasNotClosed(bg3.done) {
		t.Error(errNotFinished)
	}
}

func DependencyShutdownChainTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = withShutdown()
		bg3 = withShutdown()

		okDone1 = runShutdownable(bg1)
		okDone2 = runShutdownable(bg2)
		okDone3 = runShutdownable(bg3)
	)

	bg4 := withDependency(bg3, bg1)
	bg4 = withDependency(bg4, bg2)

	if hasClosed(
		bg1.end, bg2.end, bg3.end,
		bg1.done, bg2.done, bg3.done, bg4.finished,
	) {
		t.Error(errInitClosed)
	}

	go bg4.close()
	time.Sleep(failTimeout)

	switch {
	case hasNotClosed(bg2.end):
		t.Error(errNotClosed)
	case hasClosed(bg1.end, bg3.end):
		t.Error(errClosed)
	case hasClosed(bg1.done, bg2.done, bg3.done, bg4.finished):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone2)

	switch {
	case hasNotClosed(bg1.end):
		t.Error(errNotClosed)
	case hasNotClosed(bg2.done):
		t.Error(errNotFinished)
	case hasClosed(bg3.end):
		t.Error(errClosed)
	case hasClosed(bg1.done, bg3.done, bg4.finished):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone1)

	switch {
	case hasNotClosed(bg3.end):
		t.Error(errNotClosed)
	case hasNotClosed(bg1.done):
		t.Error(errNotFinished)
	case hasClosed(bg3.done, bg4.finished):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone3)

	if hasNotClosed(bg3.done, bg4.finished) {
		t.Error(errNotFinished)
	}
}

func DependencyShutdownSuccessiveCloseTest(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive close call caused panic")
		}
	}()

	var (
		bg1 = withShutdown()
		bg2 = withShutdown()
	)

	close(runShutdownable(bg1))
	close(runShutdownable(bg2))

	bg4 := withDependency(bg1, bg2)

	go bg4.close()
	time.Sleep(failTimeout)

	bg4.close()
}

func DependencyShutdownChildrenTimeoutTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = emptyBackground{}
		bg3 = bg1.DependsOn(bg2)
	)

	// blocked finish
	_ = runShutdownable(bg1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg3.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}
}

func DependencyShutdownParentTimeoutTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = withShutdown()
		bg3 = bg1.DependsOn(bg2)
	)

	okDone1 := runShutdownable(bg1)
	_ = runShutdownable(bg2)

	closeChanAndPropagate(okDone1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg3.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}
}

func DependencyShutdownUnclosedTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withShutdown()
		bg2 = withShutdown()
		bg3 = withDependency(bg1, bg2)
	)

	// blocked finish
	okDone2 := runShutdownable(bg2)
	okDone1 := runShutdownable(bg1)

	closeChanAndPropagate(okDone1, okDone2)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := bg3.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}

	if err := bg3.cause(); err != nil {
		t.Errorf(errNotFinished)
	}
}

func DependencyWaitTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withWait()
		bg2 = withWait()
		bg3 = withWait()

		okDone1 = runWaitable(bg1)
		okDone2 = runWaitable(bg2)
		okDone3 = runWaitable(bg3)

		bg4 = bg3.DependsOn(bg1, bg2)
	)

	done := make(chan struct{})

	go func() {
		bg4.Wait()
		close(done)
	}()

	time.Sleep(failTimeout)

	if hasClosed(done) {
		t.Error(errNotWaited)
	}

	closeChanAndPropagate(okDone1, okDone2)

	if hasClosed(done) {
		t.Error(errNotWaited)
	}

	closeChanAndPropagate(okDone3)

	if hasNotClosed(done) {
		t.Error(errNotWaited)
	}
}

func DependencyReadinessTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withReadiness()
		bg2 = withReadiness()
		bg3 = withReadiness()
	)

	bg4 := withDependency(bg3, bg1, bg2)

	if hasClosed(
		bg1.ready, bg2.ready, bg3.ready,
	) {
		t.Error(errInitReady)
	}

	bg1ReadyC := bg1.Ready()
	bg2ReadyC := bg2.Ready()
	bg3ReadyC := bg3.Ready()
	bg4ReadyC := bg4.Ready()

	bg1.Ok()

	time.Sleep(failTimeout)

	switch {
	case hasNotClosed(bg1.ready, bg1ReadyC):
		t.Error(errNotReady)
	case hasClosed(bg2.ready, bg3.ready, bg2ReadyC, bg3ReadyC, bg4ReadyC):
		t.Error(errReady)
	}

	bg2.Ok()
	time.Sleep(failTimeout)

	switch {
	case hasNotClosed(bg1.ready, bg1ReadyC, bg2.ready, bg2ReadyC):
		t.Error(errNotReady)
	case hasClosed(bg3.ready, bg3ReadyC, bg4ReadyC):
		t.Error(errReady)
	}

	bg3.Ok()
	time.Sleep(failTimeout)

	if hasNotClosed(bg1.ready, bg1ReadyC, bg2.ready, bg2ReadyC, bg3.ready, bg3ReadyC, bg4ReadyC) {
		t.Error(errNotReady)
	}
}

func DependencyErrorParentTest(t *testing.T) {
	t.Parallel()

	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")

		bg1 = withError(err1)
		bg2 = withError(err2)
		bg3 = withDependency(bg1, bg2)
	)

	err := bg3.Err()

	// err1 is higher in the tree so it must be found first
	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}

	err = bg2.Err()
	if !errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err2, err)
	}
}

func DependencyErrorChildrenTest(t *testing.T) {
	t.Parallel()

	var (
		err1 = errors.New("error1")

		bg1 = withError(err1)
		bg2 = emptyBackground{}
		bg3 = bg2.DependsOn(bg1)
	)

	err := bg3.Err()

	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}
}

func DependencyErrorNilTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = withError(nil)
		bg2 = emptyBackground{}
		bg3 = bg2.DependsOn(bg1)
	)

	if err := bg3.Err(); err != nil {
		t.Errorf("error must be nil, have '%v'", err)
	}
}

func DependencyValueParentTest(t *testing.T) {
	t.Parallel()

	var (
		testKey   = key("test_key")
		testValue = "test_value"
		bg1       = emptyBackground{}
		bg2       = emptyBackground{}
		bg3       = bg2.DependsOn(bg1)
		bg4       = withValue(testKey, testValue)
		bg5       = withDependency(bg4, bg3)
	)

	value := bg5.Value(testKey)
	if value == nil {
		t.Error("test value for test key not found")
	}

	valueTyped, ok := value.(string)
	if !ok {
		t.Error("test value for test key has wrong type")
	}

	if valueTyped != testValue {
		t.Errorf("wrong test value: want %s have %s", testValue, valueTyped)
	}
}

func DependencyValueChildrenTest(t *testing.T) {
	t.Parallel()

	var (
		testKey   = key("test_key")
		testValue = "test_value"
		bg1       = withValue(testKey, testValue)
		bg2       = emptyBackground{}
		bg3       = bg2.DependsOn(bg1)
		bg4       = emptyBackground{}
		bg5       = bg4.DependsOn(bg3)
	)

	value := bg5.Value(testKey)
	if value == nil {
		t.Error("test value for test key not found")
	}

	valueTyped, ok := value.(string)
	if !ok {
		t.Error("test value for test key has wrong type")
	}

	if valueTyped != testValue {
		t.Errorf("wrong test value: want %s have %s", testValue, valueTyped)
	}
}

func DependencyAnnotationTest(t *testing.T) {
	t.Parallel()

	var (
		bg1 = emptyBackground{}
		bg2 = emptyBackground{}
		bg3 = withAnnotation("", bg1, bg2)
		bg4 = emptyBackground{}
		bg5 = withDependency(bg3, bg4)
	)

	if bg5.parent == nil {
		t.Errorf("parent of dependency Background is nil, must be empty Background")
	}

	if bg5.parent != bg3 {
		t.Errorf("wrong parent of dependency Background")
	}

	if len(bg5.children.backgrounds) != 1 {
		t.Errorf("wrong number of dependency Background children, want 1, have %d", len(bg5.children.backgrounds))
		return
	}

	if bg5.children.backgrounds[0] != bg4 {
		t.Errorf("wrong children of dependency Background")
	}
}
