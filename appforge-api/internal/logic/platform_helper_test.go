package logic

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskSealedJSON(t *testing.T) {
	raw := `{"plain":"visible","secret":"sb1.nonce.ciphertext","nested":["sb1.a.b"]}`
	masked := maskSealedJSON(raw)
	if strings.Contains(masked, "ciphertext") || strings.Contains(masked, "sb1.") {
		t.Fatalf("sealed values were exposed: %s", masked)
	}
	if !strings.Contains(masked, `"plain":"visible"`) || strings.Count(masked, `"***"`) != 2 {
		t.Fatalf("unexpected masked JSON: %s", masked)
	}
}

func TestMaskTemplateSnapshot(t *testing.T) {
	raw := `{"productId":1,"parameterValuesJson":"{\"secret\":\"sb1.nonce.ciphertext\",\"plain\":\"visible\"}"}`
	masked := maskTemplateSnapshot(raw)
	if strings.Contains(masked, "ciphertext") || strings.Contains(masked, "sb1.") {
		t.Fatalf("template snapshot exposed ciphertext: %s", masked)
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(masked), &snapshot); err != nil {
		t.Fatal(err)
	}
	parameters, _ := snapshot["parameterValuesJson"].(string)
	if !strings.Contains(parameters, `"secret":"***"`) {
		t.Fatalf("template snapshot mask missing: %s", masked)
	}
}
