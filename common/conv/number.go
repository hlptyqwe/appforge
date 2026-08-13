package conv

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

var decimalTextPattern = regexp.MustCompile(`^[+-]?\d+(?:\.\d+)?$`)

func ParseFloatField(value string) (float64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

// ParseDecimalField parses an exact numeric field transported as a string.
// Empty input is treated as zero to preserve the existing optional-field behavior.
func ParseDecimalField(value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, nil
	}
	if !decimalTextPattern.MatchString(value) {
		return decimal.Zero, fmt.Errorf("invalid decimal value %q", value)
	}
	return decimal.NewFromString(value)
}

// ParseBoundedDecimalField parses a non-scientific decimal string and checks
// that it fits the target database DECIMAL(integerDigits+scale, scale).
// Empty input is treated as zero for optional fields.
func ParseBoundedDecimalField(value string, integerDigits, scale int) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, nil
	}
	if integerDigits < 1 || scale < 0 || !decimalTextPattern.MatchString(value) {
		return decimal.Zero, fmt.Errorf("invalid decimal value %q", value)
	}
	unsigned := strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-")
	parts := strings.SplitN(unsigned, ".", 2)
	integerPart := strings.TrimLeft(parts[0], "0")
	if integerPart == "" {
		integerPart = "0"
	}
	fractionDigits := 0
	if len(parts) == 2 {
		fractionDigits = len(parts[1])
	}
	if len(integerPart) > integerDigits || fractionDigits > scale {
		return decimal.Zero, fmt.Errorf(
			"decimal value %q exceeds DECIMAL(%d,%d)",
			value,
			integerDigits+scale,
			scale,
		)
	}
	return decimal.NewFromString(value)
}

// FloatString serializes both legacy floating-point values and exact decimals.
// New financial code should pass decimal.Decimal.
func FloatString(value any) string {
	switch v := value.(type) {
	case decimal.Decimal:
		return v.String()
	case decimal.NullDecimal:
		if v.Valid {
			return v.Decimal.String()
		}
		return ""
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	default:
		return fmt.Sprint(value)
	}
}

func NullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
