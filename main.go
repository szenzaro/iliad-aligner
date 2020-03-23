package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() {

	wordsPath := flag.String("w", "", "path to the xlsx file containing all the words")
	// tsPath := flag.String("ts", "", "path to the tmx file containing the alignment for the words in the DB")
	flag.Parse()

	// TODO: check flags
	fmt.Println(wordsPath)
	gs := []goldStandard{}
	splitIndex := 3 * len(gs) / 10 // about 30%
	trainingSet := gs[:splitIndex]
	testSet := gs[splitIndex:]

	ff := []func(edit) float64{}
	var ar aligner // TODO: put greek aligner
	alignAlg := func(p problem, w []float64) *alignment {
		return newFromWordBags(p.from, p.to).align(ar, w)
	}

	w := learn(trainingSet, 50, 10, 1.0, 0.8, ff, alignAlg)

	for _, p := range testSet {
		a := newFromWordBags(p.p.from, p.p.to)
		res := a.align(ar, w)

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

func scoreAccuracy(a, b *alignment, w []float64) float64 {
	sa, sb := a.Score(w), b.Score(w)
	return 1.0 - math.Abs(sa-sb)/math.Max(sa, sb)
}

func editsAccuracy(a, b *alignment, w []float64) float64 {
	return 0.0 // TODO: check edit presence from a to b
}

type word struct {
	text string
}
type db = map[string]word

func loadDB(path string) db {
	data := db{}
	// TODO
	return data
}

type feature func(sw, tw []word) float64

// subType,
// relativePosition,
// textDistance,
// lemmaDistance,
// posTagSimilarity,
// scholieSimilarity,

type edit interface {
	fmt.Stringer
	AddFeatures(fs ...feature)
	Features() []feature
	Score([]float64) float64
}

type wFeature struct {
	w float64
	f feature
}

type withFeatures struct {
	features []wFeature
}

func (wf *withFeatures) AddFeatures(fs ...feature) {
	for _, f := range fs {
		wf.features = append(wf.features, wFeature{0, f})
	}
}

func (wf *withFeatures) Features() []feature {
	fs := make([]feature, len(wf.features))
	for i, f := range wf.features {
		fs[i] = f.f
	}
	return fs
}

type ins struct {
	withFeatures
	w word
}

func (e *ins) Score(w []float64) float64 {
	score := 0.0
	for i, f := range e.features {
		score += w[i] * f.f([]word{}, []word{e.w})
	}
	return score
}

type del struct {
	withFeatures
	w word
}

func (e *del) Score(w []float64) float64 {
	score := 0.0
	for i, f := range e.features {
		score += w[i] * f.f([]word{e.w}, []word{})
	}
	return score
}

type eq struct {
	withFeatures
	from word
	to   word
}

func (e *eq) Score(w []float64) float64 {
	score := 0.0
	for i, f := range e.features {
		score += w[i] * f.f([]word{e.from}, []word{e.to})
	}
	return score
}

type sub struct {
	withFeatures
	from []word
	to   []word
}

func (e *sub) Score(w []float64) float64 {
	score := 0.0
	for i, f := range e.features {
		score += w[i] * f.f(e.from, e.to)
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

func (a *alignment) Score(w []float64) float64 {
	score := 0.0
	for _, e := range a.editMap {
		score += e.Score(w)
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

func (a *alignment) align(ar aligner, w []float64) *alignment { // TODO: check code or rewrite
	F := ar.next(a)
	if len(F) == 0 {
		return a
	}
	scoredAlignment := map[string]float64{}

	minScore := math.Inf(0)
	var minAlign alignment

	scored := make([]scoredAligment, len(F))

	scoredCh := make(chan scoredAligment, len(F))
	var wg sync.WaitGroup
	// start := time.Now()
	for _, x := range F { // go routine
		wg.Add(1)
		go func(al alignment) {
			scoredCh <- scoredAligment{a: al, v: al.Score()}
			wg.Done()
		}(x)
	}
	wg.Wait()
	close(scoredCh)
	i := 0
	for a := range scoredCh {
		k := fmt.Sprint(a.a.editMap)
		_, present := scoredAlignment[k]
		if !present {
			scoredAlignment[k] = a.v
		}
		if scoredAlignment[k] < minScore {
			minScore, minAlign = scoredAlignment[k], a.a
		}
		scored[i] = a
		i++
	}

	sort.SliceStable(scored, func(x, y int) bool { return scored[x].v < scored[y].v })
	// elapsed := time.Since(start)
	// fmt.Println("scored all in ", elapsed)
	// for _, v := range scored[:min(len(scored), 10)] {
	// 	fmt.Println(v.a, " - ", v.v)
	// }

	return minAlign.align(ar)
}

func learn(
	trainingProblems []goldStandard,
	N, N0 int,
	R0, r float64,
	featureFunctions []func(edit) float64,
	alignAlg func(problem, []float64) *alignment,
) []float64 {
	w := make(Vector, len(featureFunctions))
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

type problem struct {
	from, to wordsBag
}

type goldStandard struct {
	p problem
	a alignment
}

type wordsBag = map[string]word

func shuffle(vals []goldStandard) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for n := len(vals); n > 0; n-- {
		randIndex := r.Intn(n)
		vals[n-1], vals[randIndex] = vals[randIndex], vals[n-1]
	}
}

func phi(a alignment, fs []func(edit) float64) Vector {
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
