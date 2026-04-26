package searchctx

import (
	"context"
	"testing"
	"time"
)

type contextKey string

func TestDetachedWithin_PreservesValuesAndDetachesParentCancellation(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.WithValue(context.Background(), contextKey("trace_id"), "abc123"))
	defer parentCancel()

	shared, sharedCancel := DetachedWithin(parent, time.Second)
	defer sharedCancel()

	if got := shared.Value(contextKey("trace_id")); got != "abc123" {
		t.Fatalf("shared context value = %v, want %q", got, "abc123")
	}

	parentCancel()

	select {
	case <-shared.Done():
		t.Fatal("shared context was cancelled by parent cancellation")
	case <-time.After(25 * time.Millisecond):
	}

	sharedCancel()

	select {
	case <-shared.Done():
	case <-time.After(time.Second):
		t.Fatal("shared context did not observe its own cancellation")
	}
}

func TestDetachedWithin_UsesOwnTimeoutInsteadOfParentDeadline(t *testing.T) {
	parent, parentCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer parentCancel()

	parentDeadline, ok := parent.Deadline()
	if !ok {
		t.Fatal("parent context unexpectedly has no deadline")
	}

	shared, sharedCancel := DetachedWithin(parent, 120*time.Millisecond)
	defer sharedCancel()

	sharedDeadline, ok := shared.Deadline()
	if !ok {
		t.Fatal("shared context unexpectedly has no deadline")
	}
	if !sharedDeadline.After(parentDeadline.Add(50 * time.Millisecond)) {
		t.Fatalf("shared deadline %v unexpectedly tracked parent deadline %v", sharedDeadline, parentDeadline)
	}

	<-parent.Done()

	select {
	case <-shared.Done():
		t.Fatal("shared context ended when parent deadline expired")
	case <-time.After(25 * time.Millisecond):
	}

	select {
	case <-shared.Done():
		if err := shared.Err(); err != context.DeadlineExceeded {
			t.Fatalf("shared context err = %v, want %v", err, context.DeadlineExceeded)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("shared context did not time out with its own deadline")
	}
}
