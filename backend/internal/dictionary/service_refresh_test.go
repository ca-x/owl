package dictionary

import (
	"testing"
	"time"

	"owl/backend/internal/models"
)

func TestLibraryRefreshLockSerializesConcurrentRefreshes(t *testing.T) {
	svc := NewService(nil, "", "", nil, "", 0, "", false, "", "")

	firstInside := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan struct{})
	secondInside := make(chan struct{})
	secondDone := make(chan struct{})

	go func() {
		defer close(firstDone)
		_, _ = svc.withLibraryRefreshLock(func() (report *models.MaintenanceReport, err error) {
			close(firstInside)
			<-releaseFirst
			return nil, nil
		})
	}()

	select {
	case <-firstInside:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not enter critical section")
	}

	go func() {
		defer close(secondDone)
		_, _ = svc.withLibraryRefreshLock(func() (report *models.MaintenanceReport, err error) {
			close(secondInside)
			return nil, nil
		})
	}()

	select {
	case <-secondInside:
		t.Fatal("second refresh entered while first refresh still held the lock")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not finish")
	}
	select {
	case <-secondInside:
	case <-time.After(time.Second):
		t.Fatal("second refresh did not enter after first refresh finished")
	}
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second refresh did not finish")
	}
}
