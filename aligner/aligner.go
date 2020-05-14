package aligner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/szenzaro/iliad-aligner/vectors"
	"github.com/tealeg/xlsx"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

// AdditionalData store information useful when computing features
var AdditionalData map[string]interface{}

// Word contains the information about words
type Word struct {
	ID     string
	Text   string
	Lemma  string
	Tag    string
	Verse  string
	Chant  string
	Source string
}

// DB is the database of words
type DB = map[string]Word

// LoadDB retrieves all the words from the parameter paths
func LoadDB(paths []string) (DB, error) {
	data := DB{}

	for _, path := range paths {
		xlFile, err := xlsx.OpenFile(path)
		if err != nil {
			return nil, err
		}
		for _, sheet := range xlFile.Sheets {
			for i, row := range sheet.Rows {
				if i == 0 || row.Cells[0].Value == "" || row.Cells[10].Value == "" || row.Cells[4].Value != "" {
					continue
				}
				if row.Cells[0].Value == "2354" && row.Cells[2].Value == "PARA" {
					panic("asdasd") // TODO
				}
				w := Word{
					ID:     GetWordID(row.Cells[2].Value, row.Cells[0].Value), // Source.ID // TODO check id bis
					Verse:  row.Cells[10].Value,
					Chant:  row.Cells[3].Value,
					Text:   row.Cells[19].Value, // Normalized text
					Lemma:  row.Cells[20].Value,
					Tag:    row.Cells[21].Value,
					Source: row.Cells[2].Value,
				}
				data[w.ID] = w
			}
		}
	}
	return data, nil
}

// GetWordID gets an id from source and word id
func GetWordID(source, id string) string { return fmt.Sprintf("%s.%s", source, id) }

func sortID(i, j int, arr []string) bool {
	x := strings.Split(arr[i], ".")
	y := strings.Split(arr[j], ".")

	m1, _ := strconv.ParseInt(x[1], 0, 64)
	m2, _ := strconv.ParseInt(y[1], 0, 64)

	return m1 < m2
}

func (w *Word) getProblemID() string {
	return fmt.Sprintf("%s.%s", w.Chant, w.Verse)
}

// Feature represents a computable feature for the alignment
type Feature func(Edit, map[string]interface{}) float64 // func(sw, tw []word) float64

// EditType compute the score using the edit type
func EditType(e Edit, data map[string]interface{}) float64 {
	switch e.(type) {
	case *Ins:
		return 1.0
	case *Del:
		return 2.0
	case *Eq:
		return 10.0
	case *Sub:
		return 1.0
	default:
		return 0.0
	}
}

// MaxDistance combines the distances of the other features
func MaxDistance(e Edit, data map[string]interface{}) float64 {
	return multiMax(
		LexicalSimilarity(e, data),
		LemmaDistance(e, data),
		TagDistance(e, data),
		VocDistance(e, data),
		ScholieDistance(e, data),
	)
}

func getWords(e Edit) ([]Word, []Word) {
	from := []Word{}
	to := []Word{}
	switch e.(type) {
	case *Ins:
		to = []Word{e.(*Ins).W}
	case *Del:
		from = []Word{e.(*Del).W}
	case *Eq:
		from = []Word{e.(*Eq).From}
		to = []Word{e.(*Eq).To}
	case *Sub:
		from = e.(*Sub).From
		to = e.(*Sub).To
	}
	return from, to
}

// LoadScholie gets the data from the available scholies
func LoadScholie(path string) (map[string][]string, error) {
	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	d, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	var data map[string]map[string][]string
	if err := json.Unmarshal(d, &data); err != nil {
		return nil, err
	}

	sch := map[string][]string{}
	for _, verse := range data {
		for k, v := range verse {
			sch[k] = v
		}
	}
	AdditionalData["ScholieDistance"] = sch
	return sch, nil
}

// ScholieDistance computes the distance based onscholie
func ScholieDistance(e Edit, sch map[string]interface{}) float64 {
	from, to := getWords(e)
	source, target := sumWords(from), sumWords(to)
	scholie := sch["ScholieDistance"].(map[string][]string)

	entry := source.Text
	// for k := range scholie { // TODO
	// 	if levenshteinDistance(k, source.text) <= 1 {
	// 		entry = k
	// 		break
	// 	}
	// }
	// if entry == "" {
	// 	return 0.0
	// }

	// foundH := ""
	// for k := range scholie {
	// 	if levenshteinDistance(k, entry) < 3 {
	// 		foundH = k
	// 		break
	// 	}
	// }

	// if foundH == "" {
	// 	return 0.0
	// }

	if len(scholie[entry]) == 0 {
		return 0.0
	}

	mindist := math.Inf(0)
	chosen := ""
	for _, v := range scholie[entry] {
		dist := levenshteinDistance(target.Text, v)
		if dist <= mindist {
			mindist = dist
			chosen = v
		}
	}
	if chosen == "" {
		return 0.0
	}
	// a := mindist / multiMax(float64(len(target.text)), float64(len(chosen)))
	return 1.0 - mindist //mindist/multiMax(float64(len(target.text)), float64(len(chosen)))
}

// LoadVoc loads the vocabulary data
func LoadVoc(path string) (map[string][]string, error) {
	xlFile, err := xlsx.OpenFile(path)
	if err != nil {
		return nil, err
	}
	sheet := xlFile.Sheets[0]
	voc := map[string][]string{}

	getMeanings := func(s string) []string {
		m := []string{}
		for _, p := range strings.Split(s, "-") {
			m = append(m, strings.TrimSpace(p))
		}
		return m
	}

	for _, row := range sheet.Rows {
		if len(row.Cells) < 2 {
			continue
		}
		voc[row.Cells[0].Value] = append(voc[row.Cells[0].Value], getMeanings(row.Cells[1].Value)...)
	}
	AdditionalData["VocDistance"] = voc
	return voc, nil
}

func hasSameMeaning(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, w := range a {
		for _, x := range b {
			if w == x {
				return true
			}
		}
	}
	return false
}

// VocDistance computes the distance based on vocabulary data
func VocDistance(e Edit, data map[string]interface{}) float64 {
	voc := data["VocDistance"].(map[string][]string)
	switch e.(type) {
	case *Ins:
		return 0.0
	case *Del:
		return 0.0
	case *Eq:
		if hasSameMeaning(voc[e.(*Eq).From.Lemma], voc[e.(*Eq).To.Lemma]) {
			return 1.0
		}
		return 0.0
	case *Sub:
		// TODO expand for multiple words subs
		from := e.(*Sub).From
		to := e.(*Sub).To
		if len(from) == 1 && len(to) == 1 {
			if hasSameMeaning(voc[from[0].Lemma], voc[to[0].Lemma]) {
				return 1.0
			}
		}
		return 0.0
	}
	return 0.0
}

// LemmaDistance computes the distance based on the word lemma
func LemmaDistance(e Edit, data map[string]interface{}) float64 {
	from, to := getWords(e)
	source, target := sumWords(from), sumWords(to)
	lemmaV := 1 - levenshteinDistance(source.Lemma, target.Lemma)
	return lemmaV
}

// TagDistance computes the distance based on the word tag
func TagDistance(e Edit, data map[string]interface{}) float64 {
	from, to := getWords(e)
	source, target := sumWords(from), sumWords(to)
	tagV := 1 - levenshteinDistance(source.Tag, target.Tag)
	return tagV
}

// LexicalSimilarity computes the distance based on the word text
func LexicalSimilarity(e Edit, data map[string]interface{}) float64 {
	from, to := getWords(e)
	source, target := sumWords(from), sumWords(to)
	textV := 1 - levenshteinDistance(source.Text, target.Text)
	// lemmaV := 1 - levenshteinDistance(source.lemma, target.lemma)
	// tagV := 1 - levenshteinDistance(source.tag, target.tag)

	return multiMax(textV) //, lemmaV, tagV)
}

func multiMin(vs ...float64) float64 {
	min := math.Inf(1)
	for _, v := range vs {
		if v < min {
			min = v
		}
	}
	return min
}

func multiMax(vs ...float64) float64 {
	max := math.Inf(-1)
	for _, v := range vs {
		if v > max {
			max = v
		}
	}
	return max
}

func subDist(source, target []string) float64 {
	min := int(math.Min(float64(len(source)), float64(len(target))))
	max := int(math.Max(float64(len(source)), float64(len(target))))
	sumSubs := 0.0
	for i := 0; i < min; i++ {
		sumSubs += levenshteinDistance(source[i], target[i])
	}

	for i := min; i < max; i++ {
		if len(source) > i {
			sumSubs += float64(utf8.RuneCountInString(source[i]))
		}
		if len(target) > i {
			sumSubs += float64(utf8.RuneCountInString(target[i]))
		}
	}
	return sumSubs
}

func normalizedDist(source, target []string) float64 {
	sumSubs := subDist(source, target)
	var concatSource, concatTarget string
	for _, v := range source {
		concatSource += v
	}
	for _, v := range target {
		concatTarget += v
	}
	return sumSubs + levenshteinDistance(concatSource, concatTarget)
}

func sumWords(x []Word) Word {
	var text strings.Builder
	var lemma strings.Builder
	var tag strings.Builder

	for i := 0; i < len(x); i++ {
		w := x[i]
		text.WriteString(w.Text)
		lemma.WriteString(w.Lemma)
		tag.WriteString(w.Tag)
	}

	if len(x) == 0 {
		return Word{}
	}

	return Word{
		ID:     x[0].ID,
		Chant:  x[0].Chant,
		Verse:  x[0].Verse,
		Source: x[0].Source,
		Text:   text.String(),
		Lemma:  lemma.String(),
		Tag:    tag.String(),
	}
}

func levenshteinDistance(s, t string) float64 {
	return float64(levenshtein.DistanceForStrings(
		[]rune(s),
		[]rune(t),
		levenshtein.DefaultOptions,
	))
}

// subType,
// relativePosition,
// textDistance,
// lemmaDistance,
// posTagSimilarity,
// scholieSimilarity,

// Edit is the edits interface
type Edit interface {
	fmt.Stringer
	Score(fs []Feature, ws []float64, data map[string]interface{}) float64
	GetProblemID() string
}

// Ins is the insertion edit
type Ins struct {
	W Word
}

// GetProblemID of the edit
func (e *Ins) GetProblemID() string { return e.W.getProblemID() }

// GetProblemID of the edit
func (e *Del) GetProblemID() string { return e.W.getProblemID() }

// GetProblemID of the edit
func (e *Eq) GetProblemID() string { return e.From.getProblemID() }

// GetProblemID of the edit
func (e *Sub) GetProblemID() string {
	if len(e.From) == 0 && len(e.To) == 0 {
		log.Fatalln("get problem from problem with empty source")
	}
	return e.From[0].getProblemID()
}

// Score the edit
func (e *Ins) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for i, f := range fs {
		// score += w[i] * f([]word{}, []word{e.w})
		score += ws[i] * f(e, data)
	}
	return score
}

// Del is the delition edit
type Del struct {
	W Word
}

// Score the edit
func (e *Del) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e, data)
	}
	return score
}

// Eq is the equality edit
type Eq struct {
	From Word
	To   Word
}

// Score the edit
func (e *Eq) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e, data)
	}
	return score
}

// Sub is the substitution edit
type Sub struct {
	From []Word
	To   []Word
}

// Score the edit
func (e *Sub) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e, data)
	}
	return score
}

func (e *Ins) String() string {
	return fmt.Sprintf("Ins(%s)", e.W.Text)
}
func (e *Del) String() string {
	return fmt.Sprintf("Del(%s)", e.W.Text)
}
func (e *Eq) String() string {
	return fmt.Sprintf("Eq(%s , %s)", e.From.Text, e.To.Text)
}
func (e *Sub) String() string {
	var sb strings.Builder
	sb.WriteString("Sub(")
	for _, w := range e.From {
		sb.WriteString(fmt.Sprintf("%s ", w.Text))
	}
	sb.WriteString(",")
	for _, w := range e.To {
		sb.WriteString(fmt.Sprintf(" %s", w.Text))
	}
	sb.WriteString(" )")
	return sb.String()
}

// Alignment represents a words alignment
type Alignment struct {
	editMap map[Edit]Edit
}

func (a *Alignment) includes(e Edit) bool {
	for k := range a.editMap {
		if reflect.TypeOf(e) != reflect.TypeOf(k) {
			continue
		}
		switch k.(type) {
		case *Ins:
			if k.(*Ins).W.Text == e.(*Ins).W.Text {
				return true
			}
		case *Del:
			if k.(*Del).W.Text == e.(*Del).W.Text {
				return true
			}
		case *Eq:
			if k.(*Eq).From.Text == e.(*Eq).From.Text && k.(*Eq).To.Text == e.(*Eq).To.Text {
				return true
			}
		case *Sub:
			if equalSub(k.(*Sub), e.(*Sub)) {
				return true
			}
		}
	}
	return false
}

func equalSub(s, t *Sub) bool {
	m := map[string]bool{}
	for _, k := range s.From {
		m[k.ID] = true
	}
	for _, w := range t.From {
		if !m[w.ID] {
			return false
		}
	}
	for _, k := range s.To {
		m[k.ID] = true
	}
	for _, w := range t.To {
		if !m[w.ID] {
			return false
		}
	}
	return true
}

// ScoreAccuracy checks the ratio of edits considering their score
func ScoreAccuracy(a, b *Alignment, fs []Feature, w []float64, data map[string]interface{}) float64 {
	sa, sb := a.Score(fs, w, data), b.Score(fs, w, data)
	max := math.Max(sa, sb)
	if max == 0.0 {
		return 0.0
	}
	return 1.0 - math.Abs(sa-sb)/math.Max(sa, sb)
}

// EditsAccuracy checks the ratio of edits in a that are also in std (the "correct" version)
func (a *Alignment) EditsAccuracy(std *Alignment) float64 {
	n := 0
	for _, e := range std.editMap {
		if a.includes(e) {
			n++
		}
	}
	return float64(n) / float64(len(std.editMap))
}

// Score the aligment
func (a *Alignment) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for _, e := range a.editMap {
		score += e.Score(fs, ws, data)
	}
	return score
}

// Add inserts the edit in the alignment
func (a *Alignment) Add(es ...Edit) {
	for _, v := range es {
		a.editMap[v] = v
	}
}

// Phi TODO
func Phi(a *Alignment, fs []Feature, data map[string]interface{}) vectors.Vector {
	v := make(vectors.Vector, len(fs))
	for i, f := range fs {
		featureValue := 0.0
		for _, e := range a.editMap {
			featureValue += f(e, data)
		}
		v[i] = featureValue
	}
	return v
}

func new(src, target []Word) *Alignment {
	a := Alignment{
		editMap: map[Edit]Edit{},
	}
	for _, x := range src {
		a.Add(&Del{W: x})
	}
	for _, x := range target {
		a.Add(&Ins{W: x})
	}
	return &a
}

// NewFromWordBags creates an aligment from two word bags
func NewFromWordBags(from, to WordsBag) *Alignment {
	a := Alignment{
		editMap: map[Edit]Edit{},
	}
	for _, x := range from {
		a.Add(&Del{W: x})
	}
	for _, x := range to {
		a.Add(&Ins{W: x})
	}
	return &a
}

// NewFromEdits creates an aligment from edits
func NewFromEdits(es ...Edit) *Alignment {
	a := Alignment{
		editMap: map[Edit]Edit{},
	}
	a.Add(es...)
	return &a
}

// Aligner is the interface that computes the next step of an alignment
type Aligner interface {
	next(a *Alignment, subSeqLen int) []Alignment
}

func (a *Alignment) String() string {
	var sb strings.Builder
	sb.WriteString("{ ")
	for e := range a.editMap {
		sb.WriteString(e.String())
		sb.WriteString(" ")
	}
	sb.WriteString("}")
	return sb.String()
}

// Align computes the alignement using an aligner
func (a *Alignment) Align(ar Aligner, fs []Feature, ws []float64, subseqLen int, data map[string]interface{}) (*Alignment, error) {
	if len(fs) != len(ws) {
		return nil, fmt.Errorf("features and weights len mismatch")
	}
	F := ar.next(a, subseqLen)
	if len(F) == 0 {
		return a, nil
	}
	maxScore := math.Inf(-1)
	var maxAlign Alignment

	// start := time.Now()
	for _, a := range F { // go routine
		score := a.Score(fs, ws, data)
		if score > maxScore {
			maxScore = score
			maxAlign = a
		}
	}
	// elapsed := time.Since(start)
	// fmt.Println("scored all in ", elapsed)
	// for _, v := range scored[:min(len(scored), 10)] {
	// 	fmt.Println(v.a, " - ", v.v)
	// }
	return maxAlign.Align(ar, fs, ws, subseqLen, data)
}

// WordsBag represents a set of words
type WordsBag = map[string]Word

// Problem describes an alignment problem
type Problem struct {
	From, To WordsBag
}

func (p Problem) String() string {
	fromKeys := []string{}
	toKeys := []string{}
	for k := range p.From {
		fromKeys = append(fromKeys, k)
	}

	for k := range p.To {
		toKeys = append(toKeys, k)
	}
	sort.SliceStable(fromKeys, func(i, j int) bool { return sortID(i, j, fromKeys) })
	sort.SliceStable(toKeys, func(i, j int) bool { return sortID(i, j, toKeys) })

	var sb strings.Builder
	sb.WriteString("[")
	for _, k := range fromKeys {
		sb.WriteString(p.From[k].Text)
		sb.WriteString(" ")
	}
	sb.WriteString(" -> ")
	for _, k := range toKeys {
		sb.WriteString(p.To[k].Text)
		sb.WriteString(" ")
	}
	sb.WriteString("]")
	return sb.String()
}

func (a *Alignment) filter(t reflect.Type) []Edit {
	edits := []Edit{}
	for _, v := range a.editMap {
		if reflect.TypeOf(v) == t {
			edits = append(edits, v)
		}
	}
	return edits
}

func (a Alignment) clone() Alignment {
	newA := Alignment{
		editMap: map[Edit]Edit{},
	}
	for k, v := range a.editMap {
		newA.editMap[k] = v
	}
	return newA
}

func (a *Alignment) remove(es ...Edit) {
	for _, v := range es {
		delete(a.editMap, v)
	}
}
