package aligner

import (
	"testing"
)

func TestInclude(t *testing.T) {

	w := Word{ID: "1", Text: "asd"}

	a := NewFromEdits(&Ins{W: w})

	if !a.includes(&Ins{W: w}) {
		t.Error("fail")
	}
}
