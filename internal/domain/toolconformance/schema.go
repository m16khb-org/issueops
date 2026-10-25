package toolconformance

import (
	"encoding/json"
	"fmt"
	toolconformancecontract "issueops/internal/contract/toolconformance"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Classify(observation toolconformancecontract.CallObservation, schema map[string]any, expected any) (toolconformancecontract.CaseResult, error) {
	var arguments any
	if err := json.Unmarshal(observation.RawArguments, &arguments); err != nil {
		return toolconformancecontract.CaseResult{Classification: toolconformancecontract.InvalidJSON}, nil
	}
	if observation.CallCount == 0 {
		return toolconformancecontract.CaseResult{Classification: toolconformancecontract.NoCall}, nil
	}
	if observation.CallCount > 1 {
		return toolconformancecontract.CaseResult{Classification: toolconformancecontract.MultipleCalls}, nil
	}
	advertised, err := Validate(schema, arguments)
	if err != nil {
		return toolconformancecontract.CaseResult{}, err
	}
	canonical, err := Validate(ClosedProjection(schema), arguments)
	if err != nil {
		return toolconformancecontract.CaseResult{}, err
	}
	result := toolconformancecontract.CaseResult{AdvertisedValid: len(advertised) == 0, CanonicalValid: len(canonical) == 0, Diagnostics: canonical}
	if len(canonical) == 0 {
		if reflect.DeepEqual(arguments, expected) {
			result.Classification = toolconformancecontract.ExactValid
		} else {
			result.Classification = toolconformancecontract.ValidButSemanticallyDifferent
		}
		return result, nil
	}
	allUnknown := true
	for _, d := range canonical {
		if d.Code != "unknown_key" {
			allUnknown = false
		}
	}
	if allUnknown {
		result.Classification = toolconformancecontract.UnknownKey
	} else if only(canonical, "missing_required") {
		result.Classification = toolconformancecontract.MissingRequired
	} else if only(canonical, "enum_mismatch") {
		result.Classification = toolconformancecontract.EnumMismatch
	} else if only(canonical, "wrong_type") && coercible(expected, arguments) {
		result.Classification = toolconformancecontract.CoercibleTypeDrift
	} else {
		result.Classification = toolconformancecontract.NoncoercibleTypeDrift
	}
	return result, nil
}
func only(diagnostics []toolconformancecontract.Diagnostic, code string) bool {
	for _, d := range diagnostics {
		if d.Code != code {
			return false
		}
	}
	return len(diagnostics) > 0
}
func coercible(expected, actual any) bool {
	if reflect.DeepEqual(expected, actual) {
		return true
	}
	if e, ok := expected.(map[string]any); ok {
		return coercibleObject(e, actual)
	}
	if want, ok := expected.(bool); ok {
		return coercibleBool(want, actual)
	}
	if items, ok := expected.([]any); ok {
		return coercibleStringList(items, actual)
	}
	return false
}

func coercibleObject(expected map[string]any, actual any) bool {
	a, ok := actual.(map[string]any)
	if !ok || len(expected) != len(a) {
		return false
	}
	for key, want := range expected {
		got, ok := a[key]
		if !ok || !coercible(want, got) {
			return false
		}
	}
	return true
}

// coercibleBool accepts a string that parses to the expected boolean.
func coercibleBool(want bool, actual any) bool {
	text, ok := actual.(string)
	if !ok {
		return false
	}
	got, err := strconv.ParseBool(text)
	return err == nil && got == want
}

// coercibleStringList accepts a comma-separated string equal to the expected string items.
func coercibleStringList(items []any, actual any) bool {
	text, ok := actual.(string)
	if !ok {
		return false
	}
	parts := splitCSV(text)
	if len(parts) != len(items) {
		return false
	}
	for i, item := range items {
		value, ok := item.(string)
		if !ok || parts[i] != value {
			return false
		}
	}
	return true
}

func splitCSV(text string) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func ClosedProjection(in map[string]any) map[string]any { return project(in).(map[string]any) }
func project(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			out[k] = project(v)
		}
		if x["type"] == "object" {
			if _, ok := out["additionalProperties"]; !ok {
				out["additionalProperties"] = false
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = project(x[i])
		}
		return out
	default:
		return v
	}
}
func Validate(schema map[string]any, value any) ([]toolconformancecontract.Diagnostic, error) {
	if err := supported(schema); err != nil {
		return nil, err
	}
	out := []toolconformancecontract.Diagnostic{}
	validate(schema, value, "", &out)
	SortDiagnostics(out)
	return out, nil
}
func SortDiagnostics(out []toolconformancecontract.Diagnostic) {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Expected != b.Expected {
			return a.Expected < b.Expected
		}
		return a.Actual < b.Actual
	})
}
func supported(schema map[string]any) error {
	keys := make([]string, 0, len(schema))
	for key := range schema {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := supportedKeyword(key, schema[key]); err != nil {
			return err
		}
	}
	return nil
}

// supportedKeyword는 키워드 하나가 허용 목록에 있는지, 값의 모양이 올바른지 검사한다.
func supportedKeyword(key string, value any) error {
	switch key {
	case "type", "description", "enum":
		return nil
	case "properties":
		return supportedProperties(value)
	case "items":
		child, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid_schema_keyword_shape: items")
		}
		return supported(child)
	case "additionalProperties":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("invalid_schema_keyword_shape: additionalProperties")
		}
	case "required":
		if _, ok := stringValues(value); !ok {
			return fmt.Errorf("invalid_schema_keyword_shape: required")
		}
	case "minimum":
		if _, ok := numericValue(value); !ok {
			return fmt.Errorf("invalid_schema_keyword_shape: minimum")
		}
	case "maxLength":
		return supportedMaxLength(value)
	case "pattern":
		return supportedPattern(value)
	default:
		return fmt.Errorf("schema keyword is not supported: %s", key)
	}
	return nil
}

func supportedMaxLength(value any) error {
	maxLength, ok := numericValue(value)
	if !ok || maxLength < 0 || maxLength != float64(int64(maxLength)) {
		return fmt.Errorf("invalid_schema_keyword_shape: maxLength")
	}
	return nil
}

func supportedProperties(value any) error {
	properties, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid_schema_keyword_shape: properties")
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		child, ok := properties[name].(map[string]any)
		if !ok {
			return fmt.Errorf("invalid_schema_keyword_shape: properties/%s", name)
		}
		if err := supported(child); err != nil {
			return err
		}
	}
	return nil
}

func supportedPattern(value any) error {
	pattern, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid_schema_keyword_shape: pattern")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("invalid_schema_keyword_value: pattern")
	}
	return nil
}
func validate(s map[string]any, v any, p string, out *[]toolconformancecontract.Diagnostic) {
	typ, _ := s["type"].(string)
	if typ == "object" {
		validateObject(s, v, p, out)
		return
	}
	if typ == "array" {
		validateArray(s, v, p, out)
		return
	}
	if !validateEnum(s, v, p, out) {
		return
	}
	if !validateScalarType(typ, v, p, out) {
		return
	}
	validateMinimum(s, v, p, out)
	validatePattern(s, v, p, out)
	validateMaxLength(s, v, p, out)
}

func validateObject(s map[string]any, v any, p string, out *[]toolconformancecontract.Diagnostic) {
	obj, ok := v.(map[string]any)
	if !ok {
		code := "wrong_type"
		if p == "" {
			code = "root_type_mismatch"
		}
		*out = append(*out, toolconformancecontract.Diagnostic{Path: p, Code: code, Expected: "object", Actual: typeOf(v)})
		return
	}
	props, _ := s["properties"].(map[string]any)
	required, _ := stringValues(s["required"])
	for _, key := range required {
		if _, ok := obj[key]; !ok {
			*out = append(*out, toolconformancecontract.Diagnostic{Path: pointer(p, key), Code: "missing_required", Expected: "required property", Actual: "missing"})
		}
	}
	allow, present := s["additionalProperties"].(bool)
	if !present {
		allow = true
	}
	for key, item := range obj {
		child, ok := props[key]
		if !ok {
			if !allow {
				*out = append(*out, toolconformancecontract.Diagnostic{Path: pointer(p, key), Code: "unknown_key", Expected: "declared property", Actual: typeOf(item)})
			}
			continue
		}
		validate(child.(map[string]any), item, pointer(p, key), out)
	}
}

func validateArray(s map[string]any, v any, p string, out *[]toolconformancecontract.Diagnostic) {
	arr, ok := v.([]any)
	if !ok {
		*out = append(*out, toolconformancecontract.Diagnostic{Path: p, Code: "wrong_type", Expected: "array", Actual: typeOf(v)})
		return
	}
	child, _ := s["items"].(map[string]any)
	for i, item := range arr {
		validate(child, item, pointer(p, strconv.Itoa(i)), out)
	}
}

// validateEnum는 enum 키워드가 없거나 값이 일치하면 true를, 불일치 진단을 추가했으면 false를 반환한다.
//
// Catalog schemas are Go literals whose enums are []string; only []any (a
// JSON-decoded schema) was read, so the MCP server never enforced an enum
// (probe: issueops_execution action "bogus" reached the handler).
func validateEnum(s map[string]any, v any, p string, out *[]toolconformancecontract.Diagnostic) bool {
	var values []any
	switch enum := s["enum"].(type) {
	case []any:
		values = enum
	case []string:
		for _, value := range enum {
			values = append(values, value)
		}
	default:
		return true
	}
	for _, value := range values {
		if reflect.DeepEqual(value, v) {
			return true
		}
	}
	*out = append(*out, toolconformancecontract.Diagnostic{Path: p, Code: "enum_mismatch", Expected: "enum", Actual: typeOf(v)})
	return false
}

// validateScalarType는 boolean/string/number/integer 타입이 맞으면 true를, 불일치 진단을 추가했으면 false를 반환한다.
func validateScalarType(typ string, v any, p string, out *[]toolconformancecontract.Diagnostic) bool {
	if scalarTypeMatches(typ, v) {
		return true
	}
	*out = append(*out, toolconformancecontract.Diagnostic{Path: p, Code: "wrong_type", Expected: typ, Actual: typeOf(v)})
	return false
}

// scalarTypeMatches는 boolean/string/number/integer가 아닌 타입에는 항상 true를 반환한다.
func scalarTypeMatches(typ string, v any) bool {
	switch typ {
	case "boolean":
		return isBool(v)
	case "string":
		return isString(v)
	case "number":
		return isNumber(v)
	case "integer":
		return isInteger(v)
	}
	return true
}

func validateMinimum(s map[string]any, v any, p string, out *[]toolconformancecontract.Diagnostic) {
	minimum, present := s["minimum"]
	if !present {
		return
	}
	minimumValue, minimumOK := numericValue(minimum)
	actualValue, actualOK := numericValue(v)
	if minimumOK && actualOK && actualValue < minimumValue {
		*out = append(*out, toolconformancecontract.Diagnostic{
			Path: p, Code: "minimum_mismatch",
			Expected: ">= " + formatNumericValue(minimumValue), Actual: formatNumericValue(actualValue),
		})
	}
}

func validatePattern(s map[string]any, v any, p string, out *[]toolconformancecontract.Diagnostic) {
	pattern, present := s["pattern"].(string)
	if !present {
		return
	}
	value, ok := v.(string)
	if ok && !regexp.MustCompile(pattern).MatchString(value) {
		*out = append(*out, toolconformancecontract.Diagnostic{Path: p, Code: "pattern_mismatch", Expected: pattern, Actual: "string"})
	}
}

func validateMaxLength(s map[string]any, v any, p string, out *[]toolconformancecontract.Diagnostic) {
	maximum, present := s["maxLength"]
	if !present {
		return
	}
	maximumValue, maximumOK := numericValue(maximum)
	value, valueOK := v.(string)
	if !maximumOK || !valueOK {
		return
	}
	length := utf8.RuneCountInString(value)
	if float64(length) > maximumValue {
		*out = append(*out, toolconformancecontract.Diagnostic{
			Path: p, Code: "max_length_mismatch",
			Expected: "<= " + formatNumericValue(maximumValue), Actual: strconv.Itoa(length),
		})
	}
}

func stringValues(value any) ([]string, bool) {
	switch values := value.(type) {
	case nil:
		return nil, true
	case []string:
		return append([]string(nil), values...), true
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return nil, false
			}
			result = append(result, text)
		}
		return result, true
	default:
		return nil, false
	}
}
func pointer(p, k string) string {
	k = strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
	return p + "/" + k
}
func typeOf(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case float64:
		return "number"
	default:
		return "unknown"
	}
}
func isBool(v any) bool    { _, ok := v.(bool); return ok }
func isString(v any) bool  { _, ok := v.(string); return ok }
func isNumber(v any) bool  { _, ok := v.(float64); return ok }
func isInteger(v any) bool { n, ok := v.(float64); return ok && n == float64(int64(n)) }

func numericValue(value any) (float64, bool) {
	switch value := value.(type) {
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint64:
		return float64(value), true
	case float64:
		return value, true
	default:
		return 0, false
	}
}

func formatNumericValue(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
