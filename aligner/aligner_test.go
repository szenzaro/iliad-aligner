package aligner

import (
	"fmt"
	"strings"
	"testing"
)

func TestInclude(t *testing.T) {

	w := Word{ID: "1", Text: "asd"}

	a := NewFromEdits(&Ins{W: w})

	if !a.includes(&Ins{W: w}) {
		t.Error("fail")
	}
}

func TestTrie(t *testing.T) {
	sch, err := LoadScholie("../data/scholied.json")
	if err != nil {
		t.Error(err)
	}

	s := "ἡρώων"
	s = "Πηληιάδεω"
	// fmt.Println(sch.Find(s))
	// fmt.Println(sch.Find(normalizeText(s)))
	text := strings.ToLower(normalizeText(s))
	fmt.Println(sch.Find(sch.PrefixSearch(text)[0]))
	fmt.Println(sch.Find(sch.FuzzySearch(text)[0]))

	for _, v := range sch.PrefixSearch(text) {
		if x, ok := sch.Find(v); ok {
			fmt.Println(x.Meta().([]string))
		}
	}

}
