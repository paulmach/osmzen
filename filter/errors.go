package filter

import yaml "gopkg.in/yaml.v2"

// CompileError represents an errors during the compiling
// of conditions orr expressions.
type CompileError struct {
	Cause error // reported by the Compile{Expression|Condition} funcs

	// The parsed yaml that was used as input
	Input interface{}
}

// Error returns a summary of the error.
func (e *CompileError) Error() string {
	return e.Cause.Error()
}

// YAML returns the input yaml that caused the error.
func (e *CompileError) YAML() string {
	data, err := yaml.Marshal(e.Input)
	if err != nil {
		// we are remarshalling yaml, should always just work.
		panic(err)
	}

	return string(data)
}
