package owl

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Spec represents the available configuration options and their flags.
type Spec struct {
	Name     string
	Required bool // Indicates whether the configuration is required.
	Valid    bool // Indicates whether the configuration is valid.
}

// Specs represents a collection of configuration specifications.
type Specs map[string]Spec

// Define the mapping between flags and their corresponding specifications.
var allowedSpecs = map[string]func(*Spec, string, map[string]interface{}){
	SpecNameOpaque:   handleParams,
	SpecNamePlain:    handleParams,
	SpecNameSecret:   handleParams,
	SpecNamePassword: handleParams,
}

// Handler function to validate various types of input
func handleParams(spec *Spec, value string, params map[string]interface{}) {
	if strings.TrimSpace(value) != "" {
		spec.Valid = true
		if spec.Required && params != nil {
			if length, ok := params["length"].(float64); ok {
				spec.Valid = len(value) == int(length)
			}
		}
	}
}

// GenerateSpecsFromComments maps comments to configuration key specifications.
func ParseRawSpec(values map[string]string, comments map[string]string) Specs {
	// Initialize a new Specs map to store configuration specifications.
	specs := make(Specs)

	// Iterate through each key-value pair in the comments map.
	for key, value := range values {
		// Initialize a new Spec instance.
		spec := Spec{Name: SpecNameDefault}
		comment := comments[key]

		// Skip empty comments.
		if comment == "" {
			specs[key] = spec
			continue
		}

		// Split the comment into name and parameter.
		parts := strings.SplitN(comment, ":", 2)
		name := upperFirstLetter(parts[0])
		var params string
		var jsonMap map[string]interface{}

		if len(parts) > 1 {
			params = parts[1]
			bytes := []byte(params)
			jsonMap = make(map[string]interface{})

			if err := json.Unmarshal(bytes, &jsonMap); err != nil {
				_, _ = fmt.Printf("Wrong params format for %s\n", key)
			}
		}

		// Check if the comment ends with '!' to indicate that the configuration is required.
		if strings.HasSuffix(name, "!") {
			spec.Required = true
		}

		name = strings.TrimSuffix(name, "!")
		if name != "" {
			spec.Name = name
		}

		// Check if the name is recognized and apply its parameters.
		if handler, ok := allowedSpecs[spec.Name]; ok {
			handler(&spec, value, jsonMap)
		}

		// Assign the configuration specification to the key in the Specs map.
		specs[key] = spec
	}

	// Return the populated Specs map.
	return specs
}

func upperFirstLetter(s string) string {
	if len(s) == 0 {
		return s
	}

	// Convert the first character to uppercase
	// Concatenate it with the rest of the string
	return strings.ToUpper(string(s[0])) + (s[1:])
}
