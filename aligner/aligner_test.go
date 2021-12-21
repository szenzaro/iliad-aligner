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

func TestSortID(t *testing.T) {
	ids := []string{
		"x.1",
		"x.1.1",
		"x.1.2",
		"x.2",
		"x.2.0",
		"x.1.1.4",
		"x.1.3.5",
		"x.1.4",
	}

	tt := []struct {
		idx1 int
		idx2 int
		out  bool
	}{
		{0, 0, false},
		{0, 1, false},
		{1, 2, false},
		{2, 1, false},
		{0, 3, true},
		{1, 3, true},
		{3, 4, false},
		{4, 5, false},
		{5, 6, false},
	}

	for _, v := range tt {
		if sortID(v.idx1, v.idx2, ids) != v.out {
			t.Errorf("expected %v to be less than %v", ids[v.idx1], ids[v.idx2])
		}
	}
}

func TestVocDistance(t *testing.T) {
	dict, err := LoadVoc("../data/Vocabulaire_Genavensis.xlsx", "voc-dict")
	if err != nil {
		panic(err)
	}

	var e Edit
	res := VocDistance(dict)(e) // TODO: just wip test, need to be completed
	fmt.Println(res)
}
