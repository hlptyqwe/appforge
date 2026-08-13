package alert

import "time"

// DeliveryTracker keeps the delivery state for one alert key. It separates
// observation from successful delivery so a transient publisher failure is
// retried without turning every health check into a notification.
type DeliveryTracker struct {
	active                bool
	firingDelivered       bool
	deliveredFingerprint  string
	pendingFingerprint    string
	lastFiringAttemptAt   time.Time
	lastFiringDeliveredAt time.Time
	lastResolveAttemptAt  time.Time
}

func (t *DeliveryTracker) ShouldPublishFiring(
	fingerprint string,
	now time.Time,
	reminderInterval time.Duration,
	retryInterval time.Duration,
) bool {
	if !t.active {
		t.active = true
		t.firingDelivered = false
		t.deliveredFingerprint = ""
		t.pendingFingerprint = ""
		t.lastFiringAttemptAt = time.Time{}
		t.lastFiringDeliveredAt = time.Time{}
	}
	t.lastResolveAttemptAt = time.Time{}
	if !t.firingDelivered {
		return t.pendingFingerprint != fingerprint ||
			t.lastFiringAttemptAt.IsZero() ||
			now.Sub(t.lastFiringAttemptAt) >= retryInterval
	}
	if fingerprint != t.deliveredFingerprint {
		return t.pendingFingerprint != fingerprint ||
			now.Sub(t.lastFiringAttemptAt) >= retryInterval
	}
	return now.Sub(t.lastFiringDeliveredAt) >= reminderInterval &&
		now.Sub(t.lastFiringAttemptAt) >= retryInterval
}

func (t *DeliveryTracker) MarkFiringAttempt(
	fingerprint string,
	now time.Time,
	delivered bool,
) {
	t.pendingFingerprint = fingerprint
	t.lastFiringAttemptAt = now
	if delivered {
		t.firingDelivered = true
		t.deliveredFingerprint = fingerprint
		t.lastFiringDeliveredAt = now
	}
}

// ShouldPublishResolved returns true until a delivered firing event receives a
// delivered recovery event. An incident whose firing event never left the
// process is cleared silently when the observed condition recovers.
func (t *DeliveryTracker) ShouldPublishResolved(
	now time.Time,
	retryInterval time.Duration,
) bool {
	if !t.active {
		return false
	}
	if !t.firingDelivered {
		t.reset()
		return false
	}
	return t.lastResolveAttemptAt.IsZero() ||
		now.Sub(t.lastResolveAttemptAt) >= retryInterval
}

func (t *DeliveryTracker) MarkResolvedAttempt(now time.Time, delivered bool) {
	t.lastResolveAttemptAt = now
	if delivered {
		t.reset()
	}
}

func (t *DeliveryTracker) Active() bool {
	return t.active
}

func (t *DeliveryTracker) reset() {
	*t = DeliveryTracker{}
}
