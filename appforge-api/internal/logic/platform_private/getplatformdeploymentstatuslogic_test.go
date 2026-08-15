package platform_private

import (
	"context"
	"testing"
	"time"

	"appforge/common/offlinelicense"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPublicLicenseStatus(t *testing.T) {
	now := time.Now()
	status, ready := publicLicenseStatus(nil, false, now)
	if !ready || status.Status != "not_required" {
		t.Fatalf("disabled license = %+v, ready=%v", status, ready)
	}

	license := &offlinelicense.VerifiedLicense{
		Payload: offlinelicense.Payload{
			LicenseID: "license-1", Customer: "customer", DeploymentID: "deployment-1",
			DeploymentModes: []string{"private"}, NotBefore: now.Add(-time.Hour).UnixMilli(),
			NotAfter: now.Add(time.Hour).UnixMilli(), Sequence: 2,
		},
		Fingerprint: "public-fingerprint",
	}
	status, ready = publicLicenseStatus(license, true, now)
	if !ready || status.Status != "valid" || status.Fingerprint != "public-fingerprint" {
		t.Fatalf("valid license = %+v, ready=%v", status, ready)
	}

	status, ready = publicLicenseStatus(license, true, now.Add(2*time.Hour))
	if ready || status.Status != "invalid" {
		t.Fatalf("expired license = %+v, ready=%v", status, ready)
	}
}

func TestGetPlatformDeploymentStatusRequiresSystemAdministrator(t *testing.T) {
	logic := NewGetPlatformDeploymentStatusLogic(context.Background(), nil)
	_, err := logic.GetPlatformDeploymentStatus()
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %v, want PermissionDenied", status.Code(err))
	}
}
