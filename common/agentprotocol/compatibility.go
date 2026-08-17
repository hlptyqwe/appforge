// Package agentprotocol defines the immutable Local Agent protocol window
// supported by the current AppForge binaries.
package agentprotocol

import "fmt"

const (
	// Minimum is the oldest Local Agent protocol accepted for heartbeat and
	// certificate rotation by the current control plane.
	Minimum int32 = 2
	// Current is the protocol emitted by the current Local Agent and understood
	// by the current control plane.
	Current int32 = 3
	// TaskBundle is the first protocol that supports the strict build Task Bundle.
	TaskBundle int32 = 3
)

// Supported reports whether a protocol is inside the current compatibility
// window. Unsupported Agents may authenticate for heartbeat and certificate
// rotation, but must not receive new tasks.
func Supported(protocol int32) bool {
	return protocol >= Minimum && protocol <= Current
}

// CanClaimTaskBundle reports whether a protocol is both supported and capable
// of consuming the current strict Task Bundle.
func CanClaimTaskBundle(protocol int32) bool {
	return Supported(protocol) && protocol >= TaskBundle
}

// ValidateReleaseWindow prevents deployment metadata from advertising a
// protocol window that differs from the behavior compiled into the binaries.
func ValidateReleaseWindow(minimum, current int32) error {
	if minimum != Minimum || current != Current {
		return fmt.Errorf(
			"configured Local Agent protocol window %d-%d does not match binary window %d-%d",
			minimum, current, Minimum, Current,
		)
	}
	return nil
}
