package validation

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"appforge/common/conv"
)

// Validator implements go-zero's httpx.Validator using validate struct tags.
type Validator struct{}

func New() *Validator {
	return &Validator{}
}

func (v *Validator) Validate(_ *http.Request, data any) error {
	return validateValue(reflect.ValueOf(data), "")
}

func validateValue(value reflect.Value, parent string) error {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := validateValue(value.Index(i), parent); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
	default:
		return nil
	}

	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		fieldType := typ.Field(i)
		if fieldType.PkgPath != "" {
			continue
		}
		fieldValue := value.Field(i)
		name := jsonFieldName(fieldType)
		if parent != "" {
			name = parent + "." + name
		}
		if err := validateField(name, fieldValue, fieldType.Tag.Get("validate")); err != nil {
			return err
		}
		if err := validateValue(fieldValue, name); err != nil {
			return err
		}
	}
	return nil
}

func validateField(name string, value reflect.Value, tag string) error {
	if tag == "" || tag == "-" {
		return nil
	}
	for _, rule := range strings.Split(tag, ",") {
		rule = strings.TrimSpace(rule)
		switch rule {
		case "", "omitempty":
			continue
		case "required":
			if value.IsZero() {
				return fmt.Errorf("%s is required", name)
			}
		case "decimal_gt_zero", "decimal_gte_zero":
			text, ok := stringValue(value)
			if !ok {
				return fmt.Errorf("%s must be a decimal string", name)
			}
			number, err := conv.ParseDecimalField(text)
			if err != nil || (rule == "decimal_gt_zero" && !number.IsPositive()) || (rule == "decimal_gte_zero" && number.IsNegative()) {
				return fmt.Errorf("%s must be a valid decimal", name)
			}
		default:
			integerDigits, scale, ok := decimalRule(rule)
			if !ok {
				continue
			}
			text, stringOK := stringValue(value)
			if !stringOK {
				return fmt.Errorf("%s must be a decimal string", name)
			}
			if _, err := conv.ParseBoundedDecimalField(text, integerDigits, scale); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	return nil
}

func decimalRule(rule string) (integerDigits, scale int, ok bool) {
	const prefix = "decimal_"
	if !strings.HasPrefix(rule, prefix) {
		return 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(rule, prefix), "_")
	if len(parts) != 2 {
		return 0, 0, false
	}
	precision, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	scale, err = strconv.Atoi(parts[1])
	if err != nil || precision <= 0 || scale < 0 || scale > precision {
		return 0, 0, false
	}
	return precision - scale, scale, true
}

func stringValue(value reflect.Value) (string, bool) {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return "", true
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.String {
		return "", false
	}
	return value.String(), true
}

func jsonFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("json"), ",")[0]
	if name == "" || name == "-" {
		return field.Name
	}
	return name
}
