package schema

import (
	"encoding/json"
	"fmt"
	"sync"

	"embed"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed manifest.schema.json structured.schema.json relations.schema.json run-manifest.schema.json
var files embed.FS

var (
	mu       sync.Mutex
	compiled = map[string]*jsonschema.Schema{}
)

// Validate checks one of the package/run JSON documents against the checked-in schema.
func Validate(name string, body []byte) error {
	sch, err := compiledSchema(name)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return fmt.Errorf("%s is not JSON: %w", name, err)
	}
	if err := sch.Validate(value); err != nil {
		return fmt.Errorf("%s schema validation failed: %w", name, err)
	}
	return nil
}

func compiledSchema(name string) (*jsonschema.Schema, error) {
	mu.Lock()
	defer mu.Unlock()
	if sch := compiled[name]; sch != nil {
		return sch, nil
	}
	body, err := files.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	loc := "https://fmind.local/schema/" + name
	if err := c.AddResource(loc, doc); err != nil {
		return nil, err
	}
	sch, err := c.Compile(loc)
	if err != nil {
		return nil, err
	}
	compiled[name] = sch
	return sch, nil
}
