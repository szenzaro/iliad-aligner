package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tmx "github.com/szenzaro/go-tmx"
	"github.com/tealeg/xlsx"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

func main() {

	wordsPath := flag.String("w", "", "path to the xlsx file containing all the words")
	tsPath := flag.String("ts", "", "path to the tmx file containing the alignment for the words in the DB")
	flag.Parse()

	// TODO: check flags for errors or empty strings

	wordsDB, err := loadDB(*wordsPath)
	if err != nil {
		log.Fatalln(err)
	}
	gs := loadGoldStandard(*tsPath, wordsDB)

	// splitIndex := 3 * len(gs) / 10 // about 30%
	splitIndex := 1 * len(gs) / 10 // about 10%
	trainingSet := gs[:splitIndex]
	testSet := gs[splitIndex:]

	ff := []feature{
		editType,
		lexicalSimilarity,
	}
	ar := NewGreekAligner(Scholie{}) // TODO load scholie
	alignAlg := func(p problem, w []float64) *alignment {
		a, err := newFromWordBags(p.from, p.to).align(ar, ff, w)
		if err != nil {
			log.Fatalln(err)
		}
		return a
	}

	w := learn(trainingSet, 50, 10, 1.0, 0.8, ff, alignAlg)

	for _, p := range testSet {
		a := newFromWordBags(p.p.from, p.p.to)
		res, err := a.align(ar, ff, w)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)
	}

	// wordsDB := loadDB(*wordsPath)
	// for v := range editTypes {
	// 	trainingSet := loadTrainingSet(v)
	// 	trainingData, testData := trainingSet.Split(0.3)

	// 	trainer.AddFeatures(
	// 		f1,
	// 		f2,
	// 		f3,
	// 	)
	// 	values := trainter.Train(trainingSet)
	// 	edit.SetValues(values)
	// }
}

func scoreAccuracy(a, b *alignment, fs []feature, w []float64) float64 {
	sa, sb := a.Score(fs, w), b.Score(fs, w)
	return 1.0 - math.Abs(sa-sb)/math.Max(sa, sb)
}

func editsAccuracy(a, b *alignment, w []float64) float64 {
	return 0.0 // TODO: check edit presence from a to b
}

type word struct {
	ID     string
	text   string
	lemma  string
	tag    string
	verse  string
	chant  string
	source string
}
type db = map[string]word

func loadDB(path string) (db, error) {
	data := db{}
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
				panic("asdasd")
			}
			w := word{
				ID:     getWordID(row.Cells[2].Value, row.Cells[0].Value), // Source.ID
				verse:  row.Cells[10].Value,
				chant:  row.Cells[3].Value,
				text:   row.Cells[19].Value, // Normalized text
				lemma:  row.Cells[20].Value,
				tag:    row.Cells[21].Value,
				source: row.Cells[2].Value,
			}
			data[w.ID] = w
		}
	}
	return data, nil
}

func sortID(i, j int, arr []string) bool {
	x := strings.Split(arr[i], ".")
	y := strings.Split(arr[j], ".")

	m1, _ := strconv.ParseInt(x[1], 0, 64)
	m2, _ := strconv.ParseInt(y[1], 0, 64)

	return m1 < m2
}

func getWordID(source, id string) string {
	return fmt.Sprintf("%s.%s", source, id)
}

func (w *word) getProblemID() string {
	return fmt.Sprintf("%s.%s", w.chant, w.verse)
}

func loadGoldStandard(path string, words db) []goldStandard {
	problems := getProblems(words)
	data, err := tmx.Read(path)
	if err != nil {
		log.Fatalln(err)
	}

	r, err := regexp.Compile(`(.*\{(?P<first>\d+)\-\d+\}).*`)
	if err != nil {
		log.Fatalln(err)
	}

	tus := data.Body.Tu
	for _, tu := range tus {
		from := getWordsFromTuv(tu.Tuv[0], r, "HOM", words)
		to := getWordsFromTuv(tu.Tuv[1], r, "PARA", words)
		if canGetEdit(from, to) {
			e := getEditFromTu(from, to)
			problems[e.getProblemID()].a.add(e)
		}
	}

	//to array
	gs := []goldStandard{}
	for k := range problems {
		gs = append(gs, problems[k])
	}
	return gs
}

func canGetEdit(from, to []word) bool {
	isIns := len(from) == 0 && len(to) == 1
	isDel := len(from) == 1 && len(to) == 0
	isEq := len(from) == 1 && len(to) == 1 && from[0].text == to[0].text
	notEmpty := len(from) > 0 && len(to) > 0
	return isIns || isDel || isEq || notEmpty
}

func getProblems(words db) map[string]goldStandard {
	data := map[string]goldStandard{}
	for _, w := range words {
		problemID := fmt.Sprintf("%s.%s", w.chant, w.verse)
		if _, ok := data[problemID]; !ok {
			if problemID == "" || w.source == "" {
				panic("AA")
			}
			data[problemID] = goldStandard{
				ID: problemID,
				p:  problem{from: map[string]word{}, to: map[string]word{}},
				a:  newFromEdits(), // Empty alignment
			}
		}
		if w.source == "HOM" {
			data[problemID].p.from[w.ID] = w
		}
		if w.source == "PARA" {
			data[problemID].p.to[w.ID] = w
		}
	}
	return data
}

func equal(v, w word) bool {
	return v.text == w.text
}

func getID(v string, r *regexp.Regexp) string {
	submatch := r.FindStringSubmatch(v)
	if len(submatch) < 3 {
		log.Fatalln(v, submatch)
	}
	return submatch[2]
}

func getWordsFromTuv(tuv tmx.Tuv, r *regexp.Regexp, source string, words db) []word {
	parts := strings.Split(tuv.Seg.Text, " ")
	ws := []word{}
	for _, v := range parts {
		if v != "" {
			wID := getWordID(source, getID(v, r))
			if _, ok := words[wID]; ok {
				ws = append(ws, words[wID])
			}
		}
	}
	return ws
}

func getEditFromTu(from, to []word) edit {
	switch l := len(from); {
	case l == 1 && len(to) == 0:
		return &del{w: from[0]}
	case l == 0 && len(to) == 1:
		return &ins{w: to[0]}
	case l == 1 && len(to) == 1 && equal(from[0], to[0]):
		return &eq{from: from[0], to: to[0]}
	default:
		return &sub{from: from, to: to}
	}
}

type feature func(edit) float64 // func(sw, tw []word) float64

func editType(e edit) float64 {
	switch e.(type) {
	case *ins:
		return 1.0
	case *del:
		return 2.0
	case *eq:
		return 10.0
	case *sub:
		return 1.0
	default:
		return 0.0
	}
}

func lexicalSimilarity(e edit) float64 {
	from := []word{}
	to := []word{}
	switch e.(type) {
	case *ins:
		to = []word{e.(*ins).w}
	case *del:
		from = []word{e.(*del).w}
	case *eq:
		from = []word{e.(*eq).from}
		to = []word{e.(*eq).to}
	case *sub:
		from = e.(*sub).from
		to = e.(*sub).to
	}
	source, target := sumWords(from), sumWords(to)
	v := normalizedDist([]string{source.text}, []string{target.text})
	return float64(v)
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

func sumWords(x []word) word {
	var text strings.Builder
	var lemma strings.Builder
	var tag strings.Builder

	for i := 0; i < len(x); i++ {
		w := x[i]
		text.WriteString(w.text)
		lemma.WriteString(w.lemma)
		tag.WriteString(w.tag)
	}

	if len(x) == 0 {
		return word{}
	}

	return word{
		ID:     x[0].ID,
		chant:  x[0].chant,
		verse:  x[0].verse,
		source: x[0].source,
		text:   text.String(),
		lemma:  lemma.String(),
		tag:    tag.String(),
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

type edit interface {
	fmt.Stringer
	Score(fs []feature, ws []float64) float64
	getProblemID() string
}

type ins struct {
	w word
}

func (e *ins) getProblemID() string {
	return e.w.getProblemID()
}

func (e *del) getProblemID() string {
	return e.w.getProblemID()
}

func (e *eq) getProblemID() string {
	return e.from.getProblemID()
}

func (e *sub) getProblemID() string {

	if len(e.from) == 0 && len(e.to) == 0 {
		fmt.Println(e)
	}

	return e.from[0].getProblemID()
}

func (e *ins) Score(fs []feature, ws []float64) float64 {
	score := 0.0
	for i, f := range fs {
		// score += w[i] * f([]word{}, []word{e.w})
		score += ws[i] * f(e)
	}
	return score
}

type del struct {
	w word
}

func (e *del) Score(fs []feature, ws []float64) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e)
	}
	return score
}

type eq struct {
	from word
	to   word
}

func (e *eq) Score(fs []feature, ws []float64) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e)
	}
	return score
}

type sub struct {
	from []word
	to   []word
}

func (e *sub) Score(fs []feature, ws []float64) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e)
	}
	return score
}

func (e *ins) String() string {
	return fmt.Sprintf("Ins(%s)", e.w.text)
}
func (e *del) String() string {
	return fmt.Sprintf("Del(%s)", e.w.text)
}
func (e *eq) String() string {
	return fmt.Sprintf("Eq(%s , %s)", e.from.text, e.to.text)
}
func (e *sub) String() string {
	var sb strings.Builder
	sb.WriteString("Sub(")
	for _, w := range e.from {
		sb.WriteString(fmt.Sprintf("%s ", w.text))
	}
	sb.WriteString(",")
	for _, w := range e.to {
		sb.WriteString(fmt.Sprintf(" %s", w.text))
	}
	sb.WriteString(" )")
	return sb.String()
}

type alignment struct {
	editMap map[edit]edit
}

func (a *alignment) Score(fs []feature, ws []float64) float64 {
	score := 0.0
	for _, e := range a.editMap {
		score += e.Score(fs, ws)
	}
	return score
}

func (a *alignment) add(es ...edit) {
	for _, v := range es {
		a.editMap[v] = v
	}
}

func new(src, target []word) *alignment {
	a := alignment{
		editMap: map[edit]edit{},
	}
	for _, x := range src {
		a.add(&del{w: x})
	}
	for _, x := range target {
		a.add(&ins{w: x})
	}
	return &a
}

func newFromWordBags(from, to wordsBag) *alignment {
	a := alignment{
		editMap: map[edit]edit{},
	}
	for _, x := range from {
		a.add(&del{w: x})
	}
	for _, x := range to {
		a.add(&ins{w: x})
	}
	return &a
}

func newFromEdits(es ...edit) *alignment {
	a := alignment{
		editMap: map[edit]edit{},
	}
	a.add(es...)
	return &a
}

type aligner interface {
	next(a *alignment) []alignment
}

func (a *alignment) String() string {
	var sb strings.Builder
	sb.WriteString("{ ")
	for e := range a.editMap {
		sb.WriteString(e.String())
		sb.WriteString(" ")
	}
	sb.WriteString("}")
	return sb.String()
}

func (a *alignment) align(ar aligner, fs []feature, ws []float64) (*alignment, error) {
	if len(fs) != len(ws) {
		return nil, fmt.Errorf("features and weights len mismatch")
	}
	F := ar.next(a)
	if len(F) == 0 {
		return a, nil
	}
	maxScore := math.Inf(-1)
	var maxAlign alignment

	// start := time.Now()
	for _, a := range F { // go routine
		score := a.Score(fs, ws)
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
	return maxAlign.align(ar, fs, ws)
}

func learn(
	trainingProblems []goldStandard,
	N, N0 int,
	R0, r float64,
	featureFunctions []feature,
	alignAlg func(problem, []float64) *alignment,
) []float64 {
	w := make(Vector, len(featureFunctions))
	for i := range w {
		w[i] = 1.0
	}
	epochs := []Vector{}
	n := len(trainingProblems)
	R := R0
	start := time.Now()
	for i := 0; i < N; i++ {
		start := time.Now()
		R = r * R
		shuffle(trainingProblems)
		for j := 0; j < n; j++ {
			fmt.Println(j, "/", n, " -- of ", i, "/", N)
			Ej := alignAlg(trainingProblems[j].p, w)
			diff := Diff(phi(trainingProblems[j].a, featureFunctions), phi(Ej, featureFunctions)) // phi(Ej) - phi(Êj)
			w = Sum(w, diff.Scale(R))
		}
		w = w.Normalize(Norm2)
		epochs = append(epochs, w)
		elapsed := time.Since(start)
		fmt.Println("lap ", i, " finished in ", elapsed)
	}
	elapsed := time.Since(start)
	fmt.Println("trained in  ", elapsed)

	return Avg(epochs[N0:])
}

type wordsBag = map[string]word
type problem struct {
	from, to wordsBag
}
type goldStandard struct {
	ID string
	p  problem
	a  *alignment
}

func (p problem) String() string {
	fromKeys := []string{}
	toKeys := []string{}
	for k := range p.from {
		fromKeys = append(fromKeys, k)
	}

	for k := range p.to {
		toKeys = append(toKeys, k)
	}
	sort.SliceStable(fromKeys, func(i, j int) bool { return sortID(i, j, fromKeys) })
	sort.SliceStable(toKeys, func(i, j int) bool { return sortID(i, j, toKeys) })

	var sb strings.Builder
	sb.WriteString("[")
	for _, k := range fromKeys {
		sb.WriteString(p.from[k].text)
		sb.WriteString(" ")
	}
	sb.WriteString(" -> ")
	for _, k := range toKeys {
		sb.WriteString(p.to[k].text)
		sb.WriteString(" ")
	}
	sb.WriteString("]")
	return sb.String()
}

func shuffle(vals []goldStandard) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for n := len(vals); n > 0; n-- {
		randIndex := r.Intn(n)
		vals[n-1], vals[randIndex] = vals[randIndex], vals[n-1]
	}
}

func phi(a *alignment, fs []feature) Vector {
	v := make(Vector, len(fs))
	for i, f := range fs {
		featureValue := 0.0
		for _, e := range a.editMap {
			featureValue += f(e)
		}
		v[i] = featureValue
	}
	return v
}

func (a *alignment) filter(t reflect.Type) []edit {
	edits := []edit{}
	for _, v := range a.editMap {
		if reflect.TypeOf(v) == t {
			edits = append(edits, v)
		}
	}
	return edits
}

func (a alignment) clone() alignment {
	newA := alignment{
		editMap: map[edit]edit{},
	}
	for k, v := range a.editMap {
		newA.editMap[k] = v
	}
	return newA
}

func (a *alignment) remove(es ...edit) {
	for _, v := range es {
		delete(a.editMap, v)
	}
}
