package alert

import (
	"testing"
	"time"
)

func TestDeliveryTrackerFirstRetryChangeReminderAndResolve(t *testing.T) {
	var tracker DeliveryTracker
	now := time.Unix(1000, 0)
	retry := 30 * time.Second
	reminder := 30 * time.Minute

	if !tracker.ShouldPublishFiring("stale", now, reminder, retry) {
		t.Fatal("first firing must publish")
	}
	tracker.MarkFiringAttempt("stale", now, false)
	if tracker.ShouldPublishFiring("stale", now.Add(29*time.Second), reminder, retry) {
		t.Fatal("failed firing must respect retry interval")
	}
	if !tracker.ShouldPublishFiring("stale", now.Add(retry), reminder, retry) {
		t.Fatal("failed firing must retry")
	}
	tracker.MarkFiringAttempt("stale", now.Add(retry), true)
	if tracker.ShouldPublishFiring("stale", now.Add(time.Minute), reminder, retry) {
		t.Fatal("unchanged firing must be deduplicated")
	}
	if !tracker.ShouldPublishFiring("failed", now.Add(2*time.Minute), reminder, retry) {
		t.Fatal("content change must publish")
	}
	tracker.MarkFiringAttempt("failed", now.Add(2*time.Minute), true)
	if !tracker.ShouldPublishFiring("failed", now.Add(32*time.Minute), reminder, retry) {
		t.Fatal("unchanged incident must send reminder")
	}
	tracker.MarkFiringAttempt("failed", now.Add(32*time.Minute), true)

	if !tracker.ShouldPublishResolved(now.Add(33*time.Minute), retry) {
		t.Fatal("delivered firing must publish recovery")
	}
	tracker.MarkResolvedAttempt(now.Add(33*time.Minute), false)
	if tracker.ShouldPublishResolved(now.Add(33*time.Minute+29*time.Second), retry) {
		t.Fatal("failed recovery must respect retry interval")
	}
	if !tracker.ShouldPublishResolved(now.Add(33*time.Minute+retry), retry) {
		t.Fatal("failed recovery must retry")
	}
	tracker.MarkResolvedAttempt(now.Add(33*time.Minute+retry), true)
	if tracker.Active() {
		t.Fatal("delivered recovery must close incident")
	}
}

func TestDeliveryTrackerClearsUndeliveredIncidentOnRecovery(t *testing.T) {
	var tracker DeliveryTracker
	now := time.Unix(1000, 0)
	if !tracker.ShouldPublishFiring("stale", now, 30*time.Minute, 30*time.Second) {
		t.Fatal("first firing must publish")
	}
	tracker.MarkFiringAttempt("stale", now, false)
	if tracker.ShouldPublishResolved(now.Add(time.Minute), 30*time.Second) {
		t.Fatal("undelivered incident must not emit recovery")
	}
	if tracker.Active() {
		t.Fatal("undelivered recovered incident must reset")
	}
}
