package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	tmx "github.com/szenzaro/go-tmx"
	aligner "github.com/szenzaro/iliad-aligner/aligner"
	vectors "github.com/szenzaro/iliad-aligner/vectors"
	"github.com/tealeg/xlsx"
)

type goldStandard struct {
	ID string
	p  aligner.Problem
	a  *aligner.Alignment
}

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

	aligner.AdditionalData = map[string]interface{}{}

	fmt.Println("Loading vocabulary")
	_, err = aligner.LoadVoc(*vocPath)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Loading scholie")
	_, err = aligner.LoadScholie(*scholiePath)
	if err != nil {
		log.Fatalln(err)
	}

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

	ff := []aligner.Feature{
		aligner.EditType,
		aligner.LexicalSimilarity,
		aligner.LemmaDistance,
		aligner.TagDistance,
		aligner.VocDistance,
		aligner.ScholieDistance,
		aligner.MaxDistance,
	}
	ar := aligner.NewGreekAligner() // TODO load scholie
	subseqLen := 5
	alignAlg := func(p aligner.Problem, w []float64) *aligner.Alignment {
		a, err := aligner.NewFromWordBags(p.From, p.To).Align(ar, ff, w, subseqLen, aligner.AdditionalData)
		if err != nil {
			log.Fatalln(err)
		}
		return a
	}
	// w:=  []float64{0.9668163361323169, 0.14577072520289328}
	// w := learn(trainingSet, 50, 10, 1.0, 0.8, ff, alignAlg)
	fmt.Println("Start learning process...")
	startLearn := time.Now()
	w := learn(trainingSet, 50, 10, 1.0, 0.8, ff, alignAlg, aligner.AdditionalData)
	elapsedLearn := time.Since(startLearn)

	// w := learn(trainingSet, 4, 1, 1.0, 0.8, ff, alignAlg)

	fmt.Println("Align verses: ")

	totalAcc := 0.0
	totalEditAcc := 0.0
	start := time.Now()
	for i, p := range testSet {
		fmt.Println(p.ID, " ", i*100/len(testSet))
		a := aligner.NewFromWordBags(p.p.From, p.p.To)
		res, err := a.Align(ar, ff, w, subseqLen, aligner.AdditionalData)
		if err != nil {
			log.Fatalln(err)
		}
		acc := aligner.ScoreAccuracy(p.a, res, ff, w, aligner.AdditionalData)
		totalAcc += acc
		editAcc := res.EditsAccuracy(p.a)
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

func loadGoldStandard(path string, words aligner.DB) []goldStandard {
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
			problems[e.GetProblemID()].a.Add(e)
		}
	}

	//to array
	gs := []goldStandard{}
	for k := range problems {
		gs = append(gs, problems[k])
	}
	return gs
}

func learn(
	trainingProblems []goldStandard,
	N, N0 int,
	R0, r float64,
	featureFunctions []aligner.Feature,
	alignAlg func(aligner.Problem, []float64) *aligner.Alignment,
	data map[string]interface{},
) []float64 {
	w := make(vectors.Vector, len(featureFunctions))
	for i := range w {
		w[i] = 1.0
	}
	epochs := []vectors.Vector{}
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
			diff := vectors.Diff(
				aligner.Phi(trainingProblems[j].a, featureFunctions, data),
				aligner.Phi(Ej, featureFunctions, data)) // phi(Ej) - phi(Êj)
			w = vectors.Sum(w, diff.Scale(R))
			fmt.Println("finished in: ", time.Since(ss))
		}
		w = w.Normalize(vectors.Norm2)
		epochs = append(epochs, w)
		elapsed := time.Since(start)
		fmt.Println("lap ", i+1, " finished in ", elapsed)
	}
	elapsed := time.Since(start)
	fmt.Println("trained in  ", elapsed)

	return vectors.Avg(epochs[N0:])
}

func loadDB(path string) (aligner.DB, error) {
	data := aligner.DB{}
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
			w := aligner.Word{
				ID:     aligner.GetWordID(row.Cells[2].Value, row.Cells[0].Value), // Source.ID // TODO check id bis
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

func getProblems(words aligner.DB) map[string]goldStandard {
	data := map[string]goldStandard{}
	for _, w := range words {
		problemID := fmt.Sprintf("%s.%s", w.Chant, w.Verse)
		if _, ok := data[problemID]; !ok {
			if problemID == "" || w.Source == "" {
				panic("AA") // TODO
			}
			data[problemID] = goldStandard{
				ID: problemID,
				p:  aligner.Problem{From: map[string]aligner.Word{}, To: map[string]aligner.Word{}},
				a:  aligner.NewFromEdits(), // Empty alignment
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

func shuffle(vals []goldStandard) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for n := len(vals); n > 0; n-- {
		randIndex := r.Intn(n)
		vals[n-1], vals[randIndex] = vals[randIndex], vals[n-1]
	}
}

func equal(v, w aligner.Word) bool { return v.Text == w.Text }

func getID(v string, r *regexp.Regexp) string {
	submatch := r.FindStringSubmatch(v)
	if len(submatch) < 3 {
		log.Fatalln(v, submatch)
	}
	return submatch[2]
}

func getWordsFromTuv(tuv tmx.Tuv, r *regexp.Regexp, source string, words aligner.DB) []aligner.Word {
	parts := strings.Split(tuv.Seg.Text, " ")
	ws := []aligner.Word{}
	for _, v := range parts {
		if v != "" {
			wID := aligner.GetWordID(source, getID(v, r))
			if _, ok := words[wID]; ok {
				ws = append(ws, words[wID])
			}
		}
	}
	return ws
}

func getEditFromTu(from, to []aligner.Word) aligner.Edit {
	switch l := len(from); {
	case l == 1 && len(to) == 0:
		return &aligner.Del{W: from[0]}
	case l == 0 && len(to) == 1:
		return &aligner.Ins{W: to[0]}
	case l == 1 && len(to) == 1 && equal(from[0], to[0]):
		return &aligner.Eq{From: from[0], To: to[0]}
	default:
		return &aligner.Sub{From: from, To: to}
	}
}

func canGetEdit(from, to []aligner.Word) bool {
	isIns := len(from) == 0 && len(to) == 1
	isDel := len(from) == 1 && len(to) == 0
	isEq := len(from) == 1 && len(to) == 1 && from[0].Text == to[0].Text
	notEmpty := len(from) > 0 && len(to) > 0
	return isIns || isDel || isEq || notEmpty
}
