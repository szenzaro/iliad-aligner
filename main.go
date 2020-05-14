package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tmx "github.com/szenzaro/go-tmx"
	"github.com/tealeg/xlsx"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

// AdditionalData store information useful when computing features
var AdditionalData map[string]interface{}

func main() {
	f, err := os.Create("cpu_profile")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	wordsPath := flag.String("w", "", "path to the xlsx file containing all the words")
	tsPath := flag.String("ts", "", "path to the tmx file containing the alignment for the words in the DB")
	vocPath := flag.String("voc", "data/Vocabulaire_Genavensis.xlsx", "path to the vocabulary xlsx file")
	scholiePath := flag.String("sch", "data/scholied.json", "path to the scholie JSON file")

	flag.Parse()
	// TODO: check flags for errors or empty strings

	AdditionalData = map[string]interface{}{}

	fmt.Println("Loading vocabulary")
	voc, err := LoadVoc(*vocPath)
	if err != nil {
		log.Fatalln(err)
	}

	AdditionalData["VocDistance"] = voc

	fmt.Println("Loading scholie")
	scholie, err := LoadScholie(*scholiePath)
	if err != nil {
		log.Fatalln(err)
	}

	AdditionalData["ScholieDistance"] = scholie

	fmt.Println("Loading words database")
	wordsDB, err := loadDB(*wordsPath)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Loading gold standard")
	gs := loadGoldStandard(*tsPath, wordsDB)

	splitIndex := 3 * len(gs) / 10 // about 30%
	// splitIndex := 25 * len(gs) / 100 // about 25%
	trainingSet := gs[:splitIndex]
	testSet := gs[splitIndex:]

	ff := []Feature{
		EditType,
		LexicalSimilarity,
		LemmaDistance,
		TagDistance,
		VocDistance,
		ScholieDistance,
		MaxDistance,
	}
	ar := NewGreekAligner() // TODO load scholie
	subseqLen := 5
	alignAlg := func(p Problem, w []float64) *Alignment {
		a, err := NewFromWordBags(p.From, p.To).Align(ar, ff, w, subseqLen, AdditionalData)
		if err != nil {
			log.Fatalln(err)
		}
		return a
	}
	// w:=  []float64{0.9668163361323169, 0.14577072520289328}
	// w := learn(trainingSet, 50, 10, 1.0, 0.8, ff, alignAlg)
	fmt.Println("Start learning process...")
	startLearn := time.Now()
	w := learn(trainingSet, 50, 10, 1.0, 0.8, ff, alignAlg, AdditionalData)
	elapsedLearn := time.Since(startLearn)

	// w := learn(trainingSet, 4, 1, 1.0, 0.8, ff, alignAlg)

	fmt.Println("Align verses: ")

	totalAcc := 0.0
	totalEditAcc := 0.0
	start := time.Now()
	for i, p := range testSet {
		fmt.Println(p.ID, " ", i*100/len(testSet))
		a := NewFromWordBags(p.p.From, p.p.To)
		res, err := a.Align(ar, ff, w, subseqLen, AdditionalData)
		if err != nil {
			log.Fatalln(err)
		}
		acc := scoreAccuracy(p.a, res, ff, w, AdditionalData)
		totalAcc += acc
		editAcc := res.editsAccuracy(p.a)
		totalEditAcc += editAcc
		fmt.Println()
		fmt.Println("Expected: ", p.a)
		fmt.Println("Got: ", res)
		fmt.Println("with accuracy ", acc)
		fmt.Println("with edit accuracy ", editAcc)
		fmt.Println()
	}
	elapsed := time.Since(start)
	fmt.Println("Learned w: ", w)
	fmt.Println("Split percentage: ", float64(splitIndex)/float64(len(gs)))
	fmt.Println("Learn time needed: ", elapsedLearn)
	fmt.Println("Alignment time needed: ", elapsed)
	fmt.Println("With functions:")
	for _, f := range ff {
		fmt.Println("\t- ", getFunctionName(f))
	}
	fmt.Println("Total accuracy: ", totalAcc/float64(len(testSet)))
	fmt.Println("Total edit accuracy: ", totalEditAcc/float64(len(testSet)))
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func scoreAccuracy(a, b *Alignment, fs []Feature, w []float64, data map[string]interface{}) float64 {
	sa, sb := a.score(fs, w, data), b.score(fs, w, data)
	max := math.Max(sa, sb)
	if max == 0.0 {
		return 0.0
	}
	return 1.0 - math.Abs(sa-sb)/math.Max(sa, sb)
}

// editsAccuracy checks the ratio of edits in a that are also in std (the "correct" version)
func (a *Alignment) editsAccuracy(std *Alignment) float64 {
	n := 0
	for _, e := range std.editMap {
		if a.includes(e) {
			n++
		}
	}
	return float64(n) / float64(len(std.editMap))
}

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

func loadDB(path string) (DB, error) {
	data := DB{}
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
				ID:     getWordID(row.Cells[2].Value, row.Cells[0].Value), // Source.ID
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
	return data, nil
}

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
					ID:     getWordID(row.Cells[2].Value, row.Cells[0].Value), // Source.ID
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

func (w *Word) getProblemID() string {
	return fmt.Sprintf("%s.%s", w.Chant, w.Verse)
}

func loadGoldStandard(path string, words DB) []goldStandard {
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

func canGetEdit(from, to []Word) bool {
	isIns := len(from) == 0 && len(to) == 1
	isDel := len(from) == 1 && len(to) == 0
	isEq := len(from) == 1 && len(to) == 1 && from[0].Text == to[0].Text
	notEmpty := len(from) > 0 && len(to) > 0
	return isIns || isDel || isEq || notEmpty
}

func getProblems(words DB) map[string]goldStandard {
	data := map[string]goldStandard{}
	for _, w := range words {
		problemID := fmt.Sprintf("%s.%s", w.Chant, w.Verse)
		if _, ok := data[problemID]; !ok {
			if problemID == "" || w.Source == "" {
				panic("AA") // TODO
			}
			data[problemID] = goldStandard{
				ID: problemID,
				p:  Problem{From: map[string]Word{}, To: map[string]Word{}},
				a:  newFromEdits(), // Empty alignment
			}
		}
		if w.Source == "HOM" {
			data[problemID].p.From[w.ID] = w
		}
		if w.Source == "PARA" {
			data[problemID].p.To[w.ID] = w
		}
	}
	return data
}

func equal(v, w Word) bool {
	return v.Text == w.Text
}

func getID(v string, r *regexp.Regexp) string {
	submatch := r.FindStringSubmatch(v)
	if len(submatch) < 3 {
		log.Fatalln(v, submatch)
	}
	return submatch[2]
}

func getWordsFromTuv(tuv tmx.Tuv, r *regexp.Regexp, source string, words DB) []Word {
	parts := strings.Split(tuv.Seg.Text, " ")
	ws := []Word{}
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

func getEditFromTu(from, to []Word) edit {
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

// Feature represents a computable feature for the alignment
type Feature func(edit, map[string]interface{}) float64 // func(sw, tw []word) float64

func EditType(e edit, data map[string]interface{}) float64 {
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

// MaxDistance combines the distances of the other features
func MaxDistance(e edit, data map[string]interface{}) float64 {
	return multiMax(
		LexicalSimilarity(e, data),
		LemmaDistance(e, data),
		TagDistance(e, data),
		VocDistance(e, data),
		ScholieDistance(e, data),
	)
}

func getWords(e edit) ([]Word, []Word) {
	from := []Word{}
	to := []Word{}
	switch e.(type) {
	case *ins:
		to = []Word{e.(*ins).w}
	case *del:
		from = []Word{e.(*del).w}
	case *eq:
		from = []Word{e.(*eq).from}
		to = []Word{e.(*eq).to}
	case *sub:
		from = e.(*sub).from
		to = e.(*sub).to
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
	return sch, nil
}

// ScholieDistance computes the distance based onscholie
func ScholieDistance(e edit, sch map[string]interface{}) float64 {
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
func VocDistance(e edit, data map[string]interface{}) float64 {
	voc := data["VocDistance"].(map[string][]string)
	switch e.(type) {
	case *ins:
		return 0.0
	case *del:
		return 0.0
	case *eq:
		if hasSameMeaning(voc[e.(*eq).from.Lemma], voc[e.(*eq).to.Lemma]) {
			return 1.0
		}
		return 0.0
	case *sub:
		// TODO expand for multiple words subs
		from := e.(*sub).from
		to := e.(*sub).to
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
func LemmaDistance(e edit, data map[string]interface{}) float64 {
	from, to := getWords(e)
	source, target := sumWords(from), sumWords(to)
	lemmaV := 1 - levenshteinDistance(source.Lemma, target.Lemma)
	return lemmaV
}

// TagDistance computes the distance based on the word tag
func TagDistance(e edit, data map[string]interface{}) float64 {
	from, to := getWords(e)
	source, target := sumWords(from), sumWords(to)
	tagV := 1 - levenshteinDistance(source.Tag, target.Tag)
	return tagV
}

// LexicalSimilarity computes the distance based on the word text
func LexicalSimilarity(e edit, data map[string]interface{}) float64 {
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

type edit interface {
	fmt.Stringer
	Score(fs []Feature, ws []float64, data map[string]interface{}) float64
	getProblemID() string
}

type ins struct {
	w Word
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
		fmt.Println(e) // TODO
	}

	return e.from[0].getProblemID()
}

func (e *ins) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for i, f := range fs {
		// score += w[i] * f([]word{}, []word{e.w})
		score += ws[i] * f(e, data)
	}
	return score
}

type del struct {
	w Word
}

func (e *del) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e, data)
	}
	return score
}

type eq struct {
	from Word
	to   Word
}

func (e *eq) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e, data)
	}
	return score
}

type sub struct {
	from []Word
	to   []Word
}

func (e *sub) Score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for i, f := range fs {
		score += ws[i] * f(e, data)
	}
	return score
}

func (e *ins) String() string {
	return fmt.Sprintf("Ins(%s)", e.w.Text)
}
func (e *del) String() string {
	return fmt.Sprintf("Del(%s)", e.w.Text)
}
func (e *eq) String() string {
	return fmt.Sprintf("Eq(%s , %s)", e.from.Text, e.to.Text)
}
func (e *sub) String() string {
	var sb strings.Builder
	sb.WriteString("Sub(")
	for _, w := range e.from {
		sb.WriteString(fmt.Sprintf("%s ", w.Text))
	}
	sb.WriteString(",")
	for _, w := range e.to {
		sb.WriteString(fmt.Sprintf(" %s", w.Text))
	}
	sb.WriteString(" )")
	return sb.String()
}

// Alignment represents a words alignment
type Alignment struct {
	editMap map[edit]edit
}

func (a *Alignment) includes(e edit) bool {
	for k := range a.editMap {
		if reflect.TypeOf(e) != reflect.TypeOf(k) {
			continue
		}
		switch k.(type) {
		case *ins:
			if k.(*ins).w.Text == e.(*ins).w.Text {
				return true
			}
		case *del:
			if k.(*del).w.Text == e.(*del).w.Text {
				return true
			}
		case *eq:
			if k.(*eq).from.Text == e.(*eq).from.Text && k.(*eq).to.Text == e.(*eq).to.Text {
				return true
			}
		case *sub:
			if equalSub(k.(*sub), e.(*sub)) {
				return true
			}
		}
	}
	return false
}

func equalSub(s, t *sub) bool {
	m := map[string]bool{}
	for _, k := range s.from {
		m[k.ID] = true
	}
	for _, w := range t.from {
		if !m[w.ID] {
			return false
		}
	}
	for _, k := range s.to {
		m[k.ID] = true
	}
	for _, w := range t.to {
		if !m[w.ID] {
			return false
		}
	}
	return true
}

func (a *Alignment) score(fs []Feature, ws []float64, data map[string]interface{}) float64 {
	score := 0.0
	for _, e := range a.editMap {
		score += e.Score(fs, ws, data)
	}
	return score
}

func (a *Alignment) add(es ...edit) {
	for _, v := range es {
		a.editMap[v] = v
	}
}

func new(src, target []Word) *Alignment {
	a := Alignment{
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

// NewFromWordBags creates an aligment from two word bags
func NewFromWordBags(from, to WordsBag) *Alignment {
	a := Alignment{
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

func newFromEdits(es ...edit) *Alignment {
	a := Alignment{
		editMap: map[edit]edit{},
	}
	a.add(es...)
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
		score := a.score(fs, ws, data)
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

func learn(
	trainingProblems []goldStandard,
	N, N0 int,
	R0, r float64,
	featureFunctions []Feature,
	alignAlg func(Problem, []float64) *Alignment,
	data map[string]interface{},
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
			fmt.Println(j+1, "/", n, " -- of ", i+1, "/", N, " ", trainingProblems[j].ID)
			ss := time.Now()
			Ej := alignAlg(trainingProblems[j].p, w)
			diff := Diff(phi(trainingProblems[j].a, featureFunctions, data), phi(Ej, featureFunctions, data)) // phi(Ej) - phi(Êj)
			w = Sum(w, diff.Scale(R))
			fmt.Println("finished in: ", time.Since(ss))
		}
		w = w.Normalize(Norm2)
		epochs = append(epochs, w)
		elapsed := time.Since(start)
		fmt.Println("lap ", i+1, " finished in ", elapsed)
	}
	elapsed := time.Since(start)
	fmt.Println("trained in  ", elapsed)

	return Avg(epochs[N0:])
}

// WordsBag represents a set of words
type WordsBag = map[string]Word

// Problem describes an alignment problem
type Problem struct {
	From, To WordsBag
}
type goldStandard struct {
	ID string
	p  Problem
	a  *Alignment
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

func shuffle(vals []goldStandard) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for n := len(vals); n > 0; n-- {
		randIndex := r.Intn(n)
		vals[n-1], vals[randIndex] = vals[randIndex], vals[n-1]
	}
}

func phi(a *Alignment, fs []Feature, data map[string]interface{}) Vector {
	v := make(Vector, len(fs))
	for i, f := range fs {
		featureValue := 0.0
		for _, e := range a.editMap {
			featureValue += f(e, data)
		}
		v[i] = featureValue
	}
	return v
}

func (a *Alignment) filter(t reflect.Type) []edit {
	edits := []edit{}
	for _, v := range a.editMap {
		if reflect.TypeOf(v) == t {
			edits = append(edits, v)
		}
	}
	return edits
}

func (a Alignment) clone() Alignment {
	newA := Alignment{
		editMap: map[edit]edit{},
	}
	for k, v := range a.editMap {
		newA.editMap[k] = v
	}
	return newA
}

func (a *Alignment) remove(es ...edit) {
	for _, v := range es {
		delete(a.editMap, v)
	}
}
