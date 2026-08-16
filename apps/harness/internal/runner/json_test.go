package runner

import (
	"io"
	"os"
	"testing"
)

func TestJSONHelpers(t *testing.T) {
	type sample struct {
		Name string `json:"name"`
	}
	b, err := jsonMarshalIndent(sample{Name: "test"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(b) == 0 {
		t.Errorf("empty marshal")
	}
	var got sample
	if err := jsonUnmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("roundtrip: got %q", got.Name)
	}
}

func TestScannerNonNil(t *testing.T) {
	sc := bufioNewScanner(os.Stdin)
	if sc == nil {
		t.Errorf("expected scanner")
	}
}

func TestIsError(t *testing.T) {
	if !errorsIs(io.EOF, io.EOF) {
		t.Errorf("expected true")
	}
}
