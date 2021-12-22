package aligner

import (
	"fmt"
	"strings"
	"testing"

	"github.com/derekparker/trie"
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

func TestScholieDistance(t *testing.T) {

	sch := trie.New()
	sch.Add("αειδε", []string{"αδε", "λεγε"})
	sch.Add("πηληιαδεω", []string{"τουπηλεωςπαιδος"})

	feature := ScholieDistance(sch)

	ws := map[string]Word{
		"αειδε":     {Text: "αειδε"},
		"αδε":       {Text: "αδε"},
		"λεγε":      {Text: "λεγε"},
		"πηληιαδεω": {Text: "τουπηλεωςπαιδος"},
	}

	tt := []struct {
		e   Edit
		out float64
	}{
		{&Ins{W: ws["αειδε"]}, 1},
		{&Sub{From: []Word{ws["αειδε"]}, To: []Word{ws["αειδε"]}}, 0.8},
		{&Sub{From: []Word{ws["αειδε"]}, To: []Word{ws["αδε"]}}, 0},
		{&Sub{From: []Word{ws["αειδε"]}, To: []Word{ws["λεγε"]}}, 0},
	}

	for _, v := range tt {
		res := feature(v.e)
		if res != v.out {
			t.Errorf("expected %v got %v for %v", v.out, res, v.e.String())
		}
	}
}

func TestDistanceOnField(t *testing.T) {
	ws := map[string]Word{
		"a":     {Text: "a", Lemma: "a", Tag: "a"},
		"aa":    {Text: "aa", Lemma: "aa", Tag: "aa"},
		"empty": {Text: "", Lemma: "", Tag: ""},
		"à":     {Text: "à", Lemma: "à", Tag: "à"},
	}

	tt := []struct {
		e   Edit
		out float64
	}{
		{&Ins{W: ws["a"]}, 0},
		{&Del{W: ws["a"]}, 0},
		{&Del{W: ws["aa"]}, 0},
		{&Eq{From: ws["a"], To: ws["a"]}, 1},
		{&Eq{From: ws["a"], To: ws["à"]}, 0},
		{&Eq{From: ws["a"], To: ws["b"]}, 0},
		{&Sub{From: []Word{ws["a"]}, To: []Word{ws["a"]}}, 1},
		{&Sub{From: []Word{ws["empty"]}, To: []Word{ws["aa"]}}, 0},
		{&Sub{From: []Word{ws["a"]}, To: []Word{ws["aa"]}}, 0.5},
		{&Sub{From: []Word{ws["a"]}, To: []Word{ws["à"]}}, 0},
		{&Sub{From: []Word{ws["à"]}, To: []Word{ws["a"]}}, 0},
		{&Sub{From: []Word{ws["a"], ws["a"]}, To: []Word{ws["a"], ws["a"]}}, 1},
		{&Sub{From: []Word{ws["a"], ws["a"]}, To: []Word{ws["aa"]}}, 1},
	}

	feature := distanceOnField("Text")
	for _, v := range tt {
		res := feature(v.e)
		if res != v.out {
			t.Errorf("expected %v got %v for %v", v.out, res, v.e.String())
		}

	}
}

func TestEqVoc(t *testing.T) {
	voc := map[string][]string{
		"ἕννυμι":   {"ἐνδύω"},
		"οἴγνυμι":  {"ἀνοίγνυμι"},
		"ἐκπάγλως": {"ἐκπληκτικῶς", "κακῶς", "μεγάλως"},
	}

	ws := map[string]Word{
		"ἕννυμι": {Lemma: "ἕννυμι"},
		"ἐνδύω":  {Lemma: "ἐνδύω"},
		"κακῶς":  {Lemma: "κακῶς"},
	}

	feature := EqEquivTermDistance(voc)

	tt := []struct {
		e   Edit
		out float64
	}{
		{&Ins{}, 0},
		{&Sub{From: []Word{ws["ἕννυμι"]}, To: []Word{ws["ἐνδύω"]}}, 1},
		{&Sub{From: []Word{ws["ἕννυμι"]}, To: []Word{ws["ἕννυμι"]}}, 0},
		{&Eq{From: ws["ἕννυμι"], To: ws["ἐνδύω"]}, 1},
	}

	for _, v := range tt {
		res := feature(v.e)
		if res != v.out {
			t.Errorf("expected %v got %v for %v", v.out, res, v.e.String())
		}
	}
}

func TestEditToString(t *testing.T) {
	tt := []struct {
		e   Edit
		out string
	}{
		{&Ins{W: Word{Text: "a"}}, "Ins(a)"},
		{&Del{W: Word{Text: "a"}}, "Del(a)"},
		{&Eq{From: Word{Text: "a"}, To: Word{Text: "b"}}, "Eq(a , b)"},
		{&Sub{From: []Word{{Text: "a"}}, To: []Word{{Text: "b"}}}, "Sub(a , b)"},
		{&Sub{From: []Word{{Text: "a"}, {Text: "c"}}, To: []Word{{Text: "b"}, {Text: "d"}}}, "Sub(a c , b d)"},
	}
	for _, v := range tt {
		res := v.e.String()
		if res != v.out {
			t.Errorf("expected %v got %v for %v", v.out, res, v.e.String())
		}
	}
}

func TestAlignmentIncludes(t *testing.T) {
	a := NewFromEdits(
		&Ins{W: Word{Text: "a"}},
		&Del{W: Word{Text: "a"}},
		&Eq{From: Word{Text: "a"}, To: Word{Text: "b"}},
		&Sub{From: []Word{{Text: "a"}}, To: []Word{{Text: "b"}}},
	)

	tt := []struct {
		e   Edit
		out bool
	}{
		{&Ins{W: Word{Text: "a"}}, true},
		{&Ins{W: Word{Text: "b"}}, false},
		{&Del{W: Word{Text: "a"}}, true},
		{&Eq{From: Word{Text: "a"}, To: Word{Text: "b"}}, true},
		{&Sub{From: []Word{{Text: "a"}}, To: []Word{{Text: "b"}}}, true},
	}

	for _, v := range tt {
		res := a.includes(v.e)
		if res != v.out {
			t.Errorf("expected %v got %v for %v", v.out, res, v.e.String())
		}
	}

}

func TestScore(t *testing.T) {
	tt := []struct {
		a        *Alignment
		features []Feature
		ws       []float64
		out      float64
	}{
		{NewFromEdits(), []Feature{}, []float64{}, 0},
		{NewFromEdits(&Ins{W: Word{Text: "a"}}), []Feature{}, []float64{}, 0},
		{NewFromEdits(&Ins{W: Word{Text: "a"}}), []Feature{LexicalSimilarity()}, []float64{1}, 0},
		{NewFromEdits(&Sub{From: []Word{{Text: "a"}}, To: []Word{{Text: "b"}}}), []Feature{LexicalSimilarity()}, []float64{1}, 0},
		{NewFromEdits(&Sub{From: []Word{{Text: "a"}}, To: []Word{{Text: "ab"}}}), []Feature{LexicalSimilarity()}, []float64{1}, 0.5},
		{NewFromEdits(&Sub{From: []Word{{Text: "aa"}}, To: []Word{{Text: "aa"}}}), []Feature{LexicalSimilarity()}, []float64{1}, 1},
		{NewFromEdits(
			&Sub{From: []Word{{Text: "a"}}, To: []Word{{Text: "a"}}},
			&Sub{From: []Word{{Text: "b"}}, To: []Word{{Text: "c"}}},
		), []Feature{LexicalSimilarity()}, []float64{1}, 1},
	}

	for _, v := range tt {
		res := v.a.Score(v.features, v.ws, map[string]interface{}{})
		if res != v.out {
			t.Errorf("expected %v got %v for %v", v.out, res, v.a.String())
		}
	}

}
