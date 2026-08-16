package web

import (
	"encoding/json"
	"io"
)

// jsonEncode writes v to w as JSON. Kept as a function so handlers can marshal
// through a single path and tests can swap it if needed.
func jsonEncode(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}
