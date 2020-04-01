package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"time"

	tmx "github.com/szenzaro/go-tmx"
)

func main() {

	wordsPath := flag.String("w", "", "path to the xlsx file containing all the words")
	tsPath := flag.String("ts", "", "path to the tmx file containing the alignment for the words in the DB")
	flag.Parse()

	// TODO: check flags
	wordsDB := loadDB(*wordsPath)
	gs := loadGoldStandard(*tsPath, &wordsDB)
	splitIndex := 3 * len(gs) / 10 // about 30%
	trainingSet := gs[:splitIndex]
	testSet := gs[splitIndex:]

	ff := []feature{
		editType,
	}
	var ar aligner // TODO: put greek aligner
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
	verse  int
	chant  int
	source string
}
type db = map[string]word

func loadDB(path string) db {
	data := db{}
	// TODO

	return data
}

func (w *word) getProblemID() string {
	return fmt.Sprintf("%s.%d.%d", w.source, w.chant, w.verse)
}

func loadGoldStandard(path string, words *db) []goldStandard {

	problems := getProblems(*words)

	// TODO
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
		e := getEditFromTu(tu, r)
		problems[e.getProblemID()].a.add(e)
	}

	//to array
	gs := make([]goldStandard, len(problems))
	for _, v := range problems {
		gs = append(gs, v)
	}
	return gs
}

func getProblems(words db) map[string]goldStandard {
	// TODO
	return map[string]goldStandard{}
}

func equal(v, w word) bool {
	return v.text == w.text
}

func getWordsFromTuv(tub tmx.Tuv, r *regexp.Regexp) []word {
	return []word{} // TODO
}

func getEditFromTu(tu tmx.Tu, r *regexp.Regexp) edit {
	from := getWordsFromTuv(tu.Tuv[0], r)
	to := getWordsFromTuv(tu.Tuv[1], r)

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
	return fmt.Sprintf("Ins( %s )", e.w.text)
}
func (e *del) String() string {
	return fmt.Sprintf("Del( %s )", e.w.text)
}
func (e *eq) String() string {
	return fmt.Sprintf("Eq( %s , %s )", e.from.text, e.to.text)
}
func (e *sub) String() string {
	var sb strings.Builder
	sb.WriteString("Sub(")
	for _, w := range e.from {
		sb.WriteString(fmt.Sprintf(" %s", w.text))
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
	for i := 0; i < N; i++ {
		R = r * R
		shuffle(trainingProblems)
		for j := 0; j < n; j++ {
			Ej := alignAlg(trainingProblems[j].p, w)
			diff := Diff(phi(trainingProblems[j].a, featureFunctions), phi(Ej, featureFunctions)) // phi(Ej) - phi(Êj)
			w = Sum(w, diff.Scale(R))
		}
		w = w.Normalize(Norm2)
		epochs[i] = w
	}

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
