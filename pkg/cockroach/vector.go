// vector.go: encode/decode gia tri VECTOR de dung trong cau truy van CockroachDB.
package cockroach

import (
	"fmt"
	"strconv"
	"strings"
)

func EncodeVector(embedding []float32) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 32)
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ","))
}
