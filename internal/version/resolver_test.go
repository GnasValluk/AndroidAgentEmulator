package version

import (
	"context"
	"testing"
)

func TestResolveAutoUsesMatrix(t *testing.T) {
	v, err := New().Resolve(context.Background(), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if v.Version == "" || v.APILevel == "" || v.Branch == "" {
		t.Fatalf("incomplete resolve: %+v", v)
	}
}

func TestResolveAcceptsAPIToo(t *testing.T) {
	v, err := New().Resolve(context.Background(), "36")
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "16" {
		t.Fatalf("api 36 should map to version 16, got %+v", v)
	}
}
