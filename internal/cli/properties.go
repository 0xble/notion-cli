package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type PropertyMode string

const (
	PropertyModeOff    PropertyMode = "off"
	PropertyModeWarn   PropertyMode = "warn"
	PropertyModeStrict PropertyMode = "strict"
)

var (
	intPattern   = regexp.MustCompile(`^-?\d+$`)
	floatPattern = regexp.MustCompile(`^-?\d+\.\d+$`)
)

func ParsePropertyMode(raw string) (PropertyMode, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch PropertyMode(mode) {
	case PropertyModeOff, PropertyModeWarn, PropertyModeStrict:
		return PropertyMode(mode), nil
	default:
		return "", fmt.Errorf("invalid --property-mode %q (expected off, warn, or strict)", raw)
	}
}

// ParseFrontmatterProperties extracts top-level YAML keys from frontmatter as
// property candidates, excluding reserved sync metadata keys.
func ParseFrontmatterProperties(content string) (map[string]any, error) {
	fmBlock := extractFrontmatterBlock(content)
	if fmBlock == "" {
		return map[string]any{}, nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(fmBlock), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter properties: %w", err)
	}

	props := make(map[string]any)
	for key, value := range raw {
		normalizedKey := strings.TrimSpace(key)
		lowerKey := strings.ToLower(normalizedKey)
		switch lowerKey {
		case "notion-id", "notion":
			continue
		case "properties", "props":
			nested, err := toStringAnyMap(value)
			if err != nil {
				return nil, fmt.Errorf("frontmatter %q must be a map: %w", key, err)
			}
			for nk, nv := range nested {
				props[nk] = normalizeYAMLValue(nv)
			}
		default:
			props[normalizedKey] = normalizeYAMLValue(value)
		}
	}

	return props, nil
}

// ParsePropertiesFlags parses semicolon-delimited --props values and repeatable
// --prop key=value assignments. Later assignments override earlier ones.
func ParsePropertiesFlags(propsValues []string, propValues []string) (map[string]any, []error) {
	out := make(map[string]any)
	var errs []error

	for _, entry := range propsValues {
		chunks := strings.Split(entry, ";")
		for _, chunk := range chunks {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				continue
			}
			key, value, err := parsePropertyAssignment(chunk)
			if err != nil {
				errs = append(errs, fmt.Errorf("invalid --props assignment %q: %w", chunk, err))
				continue
			}
			out[key] = value
		}
	}

	for _, assignment := range propValues {
		assignment = strings.TrimSpace(assignment)
		if assignment == "" {
			continue
		}
		key, value, err := parsePropertyAssignment(assignment)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid --prop assignment %q: %w", assignment, err))
			continue
		}
		out[key] = value
	}

	return out, errs
}

func MergeProperties(base map[string]any, overrides map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

func parsePropertyAssignment(raw string) (string, any, error) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok {
		return "", nil, fmt.Errorf("expected key=value")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil, fmt.Errorf("property name cannot be empty")
	}
	return key, parsePropertyValue(strings.TrimSpace(value)), nil
}

func parsePropertyValue(raw string) any {
	if raw == "" {
		return ""
	}

	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	if lower == "true" {
		return true
	}
	if lower == "false" {
		return false
	}

	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
	}

	if intPattern.MatchString(trimmed) {
		if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return i
		}
	}

	if floatPattern.MatchString(trimmed) {
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return f
		}
	}

	if len(trimmed) >= 2 {
		if (strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"")) ||
			(strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'")) {
			return trimmed[1 : len(trimmed)-1]
		}
	}

	return raw
}

func toStringAnyMap(v any) (map[string]any, error) {
	rawMap, ok := v.(map[string]any)
	if ok {
		return rawMap, nil
	}

	altMap, ok := v.(map[any]any)
	if !ok {
		return nil, fmt.Errorf("got %T", v)
	}

	out := make(map[string]any, len(altMap))
	for k, val := range altMap {
		key, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("non-string key type %T", k)
		}
		out[key] = val
	}
	return out, nil
}

func normalizeYAMLValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, val := range typed {
			out[k] = normalizeYAMLValue(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for k, val := range typed {
			key := fmt.Sprintf("%v", k)
			out[key] = normalizeYAMLValue(val)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, val := range typed {
			out[i] = normalizeYAMLValue(val)
		}
		return out
	default:
		return typed
	}
}
