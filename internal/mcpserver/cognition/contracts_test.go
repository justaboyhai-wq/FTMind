package cognition

import (
	"github.com/justaboyhai-wq/fmind/internal/types"
	"testing"
	"time"
)

func TestValidateRequestRejectsMissingBinding(t *testing.T) {
	if err := ValidateRequest(Request{Tool: ToolMemorySearch}); err == nil {
		t.Fatal("expected binding validation error")
	}
}
func TestValidateRequestAcceptsScopedBinding(t *testing.T) {
	err := ValidateRequest(Request{Tool: ToolMemorySearch, Binding: types.BindingContext{TenantID: 1, BindingID: "b", ExpiresAt: time.Now().Add(time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
}
