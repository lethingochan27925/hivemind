// vector.go: encode/decode gia tri VECTOR de dung trong cau truy van CockroachDB.
package cockroach

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
)

// EncodeVector turns an embedding into a CockroachDB VECTOR literal. NaN/±Inf
// are substituted with 0 before encoding: strconv.FormatFloat renders them as
// the strings "NaN"/"+Inf"/"-Inf", which are not valid numeric literals
// inside a VECTOR(...) constructor - an embedding that ever contained one
// (a malformed Titan response, a division-by-zero deep in some future
// embedding path) would otherwise produce a literal CockroachDB rejects with
// a generic parse error, giving no hint that the real cause was upstream in
// the embedding itself, at the one write every episodic memory depends on.
func EncodeVector(embedding []float32) string {
	parts := make([]string, len(embedding))
	nonFinite := 0
	for i, v := range embedding {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			v = 0
			nonFinite++
		}
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 32)
	}
	if nonFinite > 0 {
		log.Printf("[warn] EncodeVector: replaced %d non-finite value(s) (NaN/Inf) with 0", nonFinite)
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ","))
}
