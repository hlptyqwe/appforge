package reqenc

import (
	"net/http"
	"strings"
)

type Rule struct {
	Method       string
	Path         string
	PathPrefix   bool
	Location     Location
	PathTemplate string
	// RequiredOnly marks a catch-all rule that is enabled only in REQUIRED mode.
	// Regular rules are enabled in OPTIONAL and REQUIRED modes.
	RequiredOnly bool
	// Exempt keeps bootstrap or protocol-incompatible routes in plaintext.
	Exempt bool
}

type Registry struct {
	rules       map[string]Rule
	prefixRules []Rule
}

func NewRegistry(rules ...Rule) *Registry {
	registry := &Registry{rules: make(map[string]Rule, len(rules))}
	for _, rule := range rules {
		rule.Method = strings.ToUpper(strings.TrimSpace(rule.Method))
		if rule.PathPrefix {
			registry.prefixRules = append(registry.prefixRules, rule)
			continue
		}
		registry.rules[rule.Method+" "+rule.Path] = rule
	}
	return registry
}

func (r *Registry) Match(req *http.Request) (Rule, bool) {
	if r == nil {
		return Rule{}, false
	}
	rule, ok := r.rules[strings.ToUpper(req.Method)+" "+req.URL.Path]
	if ok {
		return rule, true
	}
	method := strings.ToUpper(req.Method)
	for _, prefixRule := range r.prefixRules {
		if prefixRule.Method == method && strings.HasPrefix(req.URL.Path, prefixRule.Path) {
			return prefixRule, true
		}
	}
	return Rule{}, false
}
