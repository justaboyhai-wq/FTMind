package schema

import "testing"

func TestValidatePackageSchemas(t *testing.T) {
	if err := Validate("structured.schema.json", []byte(`{"id":1,"title":"policy","official":{}}`)); err != nil {
		t.Fatal(err)
	}
	if err := Validate("relations.schema.json", []byte(`[{"source_id":1,"relation_type":"text_interpretation","target_url":"https://example.com"}]`)); err != nil {
		t.Fatal(err)
	}
	if err := Validate("structured.schema.json", []byte(`{"id":"bad"}`)); err == nil {
		t.Fatal("expected schema validation error")
	}
}
