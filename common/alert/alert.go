package alert

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TypeContractReconciliation = "contract_reconciliation"
	TypePriceEngineInput       = "price_engine_input"
	TypeSnapshotOutbox         = "snapshot_outbox"

	StateFiring   = "firing"
	StateResolved = "resolved"
)

type Alert struct {
	ID        string
	Type      string
	State     string
	Severity  string
	Title     string
	Message   string
	Source    string
	Key       string
	TenantID  int64
	Data      map[string]any
	CreatedAt int64
}

func New(alertType, state, severity, source, key, title, message string, now int64) Alert {
	if now <= 0 {
		now = time.Now().UnixMilli()
	}
	return Alert{
		ID:        fmt.Sprintf("%s:%s:%s:%d", alertType, key, state, now),
		Type:      alertType,
		State:     state,
		Severity:  severity,
		Title:     title,
		Message:   message,
		Source:    source,
		Key:       key,
		CreatedAt: now,
	}
}

func (a Alert) Validate() error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.Type) == "" ||
		strings.TrimSpace(a.State) == "" || strings.TrimSpace(a.Severity) == "" ||
		strings.TrimSpace(a.Title) == "" || strings.TrimSpace(a.Message) == "" ||
		strings.TrimSpace(a.Source) == "" || strings.TrimSpace(a.Key) == "" ||
		a.CreatedAt <= 0 {
		return errors.New("operational alert identity, content, source, key, and time are required")
	}
	if a.State != StateFiring && a.State != StateResolved {
		return errors.New("operational alert state is invalid")
	}
	return nil
}
