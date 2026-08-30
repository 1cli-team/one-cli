package preset

import "fmt"

// ResolveEnvWithFlag combines a preset env code with an explicit
// --env-provider flag value. Empty values mean that source made no choice.
func ResolveEnvWithFlag(presetValue, flagValue string) (string, error) {
	if flagValue == "" {
		return presetValue, nil
	}
	if presetValue == "" {
		return flagValue, nil
	}
	if presetValue != flagValue {
		return "", &EnvConflictError{Preset: presetValue, Flag: flagValue}
	}
	return presetValue, nil
}

// EnvConflictError identifies a reproducibility conflict between the encoded
// preset and an explicit CLI override.
type EnvConflictError struct {
	Preset string
	Flag   string
}

func (e *EnvConflictError) Error() string {
	return fmt.Sprintf("preset declared env=%q but --env-provider %q was also passed", e.Preset, e.Flag)
}
