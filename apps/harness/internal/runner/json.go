package runner

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

func jsonMarshalIndent(v any) ([]byte, error)  { return json.MarshalIndent(v, "", "  ") }
func jsonUnmarshal(b []byte, v any) error      { return json.Unmarshal(b, v) }
func errorsIs(err, target error) bool         { return errors.Is(err, target) }
func bufioNewScanner(r io.Reader) *bufio.Scanner { return bufio.NewScanner(r) }
