/** Stores and returns files LumiHub cannot parse */
package passthrough

import (
	"context"
	"io"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
)

type module struct{}

func New() format.Module { return module{} }

func (module) ID() string { return "passthrough" }

/** Always false. The registry uses this module when nothing else matches. */
func (module) Detect(string, []byte) bool { return false }

func (module) Parse(context.Context, io.Reader) (format.Parsed, error) {
	return format.Parsed{Format: "unknown"}, nil
}
