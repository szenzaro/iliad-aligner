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

func TestSameMeaning(t *testing.T) {
	dict := map[string][]string{
		"ἔκπαγλος": {"effrayant", "terrible", "étonnant", "merveilleux"},
		"φοβερός":  {"effrayant", "terrible", "craintif"},
		"ἐπόμνυμι": {"jurer en outre", "confirmer ce qu'on dit par un serment", "jurer à la suite"},
	}

	tt := []struct {
		w1  string
		w2  string
		out bool
	}{
		{"ἔκπαγλος", "ἔκπαγλος", true},
		{"ἔκπαγλος", "φοβερός", true},
		{"ἔκπαγλος", "ἐπόμνυμι", false},
	}

	for _, v := range tt {
		if hasSameMeaning(dict[v.w1], dict[v.w2]) != v.out {
			t.Errorf("expected hasSameMeaning of %v and %v to be %v", v.w1, v.w2, v.out)
		}
	}
}

func TestVocDistance(t *testing.T) {
	dict := map[string][]string{
		"ἔκπαγλος": {"effrayant", "terrible", "étonnant", "merveilleux"},
		"φοβερός":  {"effrayant", "terrible", "craintif"},
		"ἐπόμνυμι": {"jurer en outre", "confirmer ce qu'on dit par un serment", "jurer à la suite"},
	}

	ws := map[string]Word{
		"ἔκπαγλος": {Text: "ἔκπαγλος", Lemma: "ἔκπαγλος"},
		"φοβερός":  {Text: "φοβερός", Lemma: "φοβερός"},
		"ἐπόμνυμι": {Text: "ἐπόμνυμι", Lemma: "ἐπόμνυμι"},
	}

	edit := &Ins{W: ws["ἔκπαγλος"]}
	tt := []struct {
		e   Edit
		out float64
	}{
		{edit, 0},
		{edit, 0},
		{&Del{W: ws["ἔκπαγλος"]}, 0},
		{&Eq{From: ws["ἔκπαγλος"], To: ws["ἔκπαγλος"]}, 1},
		{&Eq{From: ws["ἔκπαγλος"], To: ws["φοβερός"]}, 1},
		{&Eq{From: ws["ἔκπαγλος"], To: ws["ἐπόμνυμι"]}, 0},
		{&Sub{From: []Word{ws["ἔκπαγλος"], ws["φοβερός"]}, To: []Word{ws["ἔκπαγλος"]}}, 1},
		{&Sub{From: []Word{ws["ἔκπαγλος"], ws["φοβερός"]}, To: []Word{ws["φοβερός"]}}, 1},
		{&Sub{From: []Word{ws["ἔκπαγλος"], ws["ἐπόμνυμι"]}, To: []Word{ws["φοβερός"]}}, 1},
		{&Sub{From: []Word{ws["ἐπόμνυμι"]}, To: []Word{ws["φοβερός"]}}, 0},
		{&Sub{From: []Word{ws["ἔκπαγλος"], ws["ἐπόμνυμι"]}, To: []Word{ws["ἐπόμνυμι"]}}, 1},
		{&Sub{From: []Word{ws["ἔκπαγλος"], ws["φοβερός"]}, To: []Word{}}, 0},
	}

	feature := VocDistance(dict)
	for _, v := range tt {
		res := feature(v.e)
		if res != v.out {
			t.Errorf("expected %v got %v with edit %v", v.out, res, v.e.String())
		}
	}
}
