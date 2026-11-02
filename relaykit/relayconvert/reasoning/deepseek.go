package reasoning

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type DeepSeekModifiers struct {
	Model       string
	Thinking    string
	Effort      string
	Temperature *float64
}

func ParseDeepSeekModifiers(model string) (DeepSeekModifiers, error) {
	result := DeepSeekModifiers{Model: model}
	base, tail, found := strings.Cut(model, "@")
	if !found {
		return result, nil
	}
	seen := make(map[string]bool)
	for _, segment := range strings.Split(tail, "@") {
		key, value, ok := strings.Cut(segment, ":")
		key = strings.ToLower(key)
		value = strings.ToLower(strings.TrimSpace(value))
		if !ok || seen[key] || base == "" {
			return result, fmt.Errorf("invalid DeepSeek model modifier %q", segment)
		}
		seen[key] = true
		switch key {
		case "thinking":
			switch value {
			case "on", "adaptive":
				result.Thinking = "enabled"
			case "off":
				result.Thinking = "disabled"
			default:
				budget, err := strconv.Atoi(value)
				if err != nil || budget < -1 {
					return result, fmt.Errorf("invalid thinking modifier %q", value)
				}
				result.Thinking = "enabled"
				switch {
				case budget == 0:
					result.Thinking = "disabled"
				case budget == -1 || budget > 8192:
					result.Effort = "high"
				case budget <= 1024:
					result.Effort = "low"
				default:
					result.Effort = "medium"
				}
			}
		case "effort":
			switch value {
			case "none":
				result.Thinking = "disabled"
			case "minimal", "low", "medium", "high", "xhigh", "max":
				result.Thinking = "enabled"
				result.Effort = value
			default:
				return result, fmt.Errorf("invalid effort modifier %q", value)
			}
		case "temperature":
			temperature, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(temperature) || math.IsInf(temperature, 0) || temperature < 0 || temperature > 2 {
				return result, fmt.Errorf("invalid temperature modifier %q", value)
			}
			result.Temperature = &temperature
		default:
			return result, fmt.Errorf("unsupported DeepSeek model modifier %q", key)
		}
	}
	if seen["thinking"] && seen["effort"] {
		return result, fmt.Errorf("thinking and effort modifiers cannot be combined")
	}
	if result.Thinking == "disabled" {
		result.Effort = ""
	}
	result.Model = base
	return result, nil
}
