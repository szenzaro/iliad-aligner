package aligner

import (
	"reflect"
	"sort"
)

type greekAligner struct{}

// NewGreekAligner creates a greek aligner
func NewGreekAligner() *greekAligner {
	return &greekAligner{}
}

func (*greekAligner) next(a *Alignment, subSeqLen int) []Alignment {
	nextAlignments := []Alignment{}

	dels := a.filter(reflect.TypeOf(&Del{}))
	if len(dels) == 0 {
		return nextAlignments
	}
	inss := a.filter(reflect.TypeOf(&Ins{}))

	// Add Eqs
	for _, x := range dels {
		x := x.(*Del)
		for _, y := range inss {
			y := y.(*Ins)
			noPunctText := RemovePunctuation(x.W.Text)
			if noPunctText != RemovePunctuation(y.W.Text) {
				continue
			}
			eq := Eq{
				From: x.W,
				To:   y.W,
			}
			newAlign := a.clone()
			newAlign.remove(x, y)
			newAlign.Add(&eq)
			nextAlignments = append(nextAlignments, newAlign)
		}
	}

	if len(nextAlignments) > 0 {
		return nextAlignments
	}

	sort.SliceStable(dels, func(x, y int) bool { return dels[x].(*Del).W.ID < dels[y].(*Del).W.ID })
	sort.SliceStable(inss, func(x, y int) bool { return inss[x].(*Ins).W.ID < inss[y].(*Ins).W.ID })

	delWords := []Word{}
	for _, x := range dels {
		delWords = append(delWords, x.(*Del).W)
	}
	subDels := limitedSubsequences(delWords, subSeqLen)

	insWords := []Word{}
	for _, x := range inss {
		insWords = append(insWords, x.(*Ins).W)
	}
	subIns := limitedSubsequences(insWords, subSeqLen)

	for _, d := range subDels {
		for _, i := range subIns {
			newSubEdit := Sub{
				From: d,
				To:   i,
			}
			newAlignment := a.clone()
			removeEditWithWordsByID(&newAlignment, append(d, i...)...)
			newAlignment.Add(&newSubEdit)

			nextAlignments = append(nextAlignments, newAlignment)
		}
	}
	return nextAlignments
}

// Scholie data
type Scholie map[string]map[string][]string

func limitedSubsequences(arr []Word, limit int) [][]Word {
	comb := func(n, m int, emit func([]int)) {
		s := make([]int, m)
		last := m - 1
		var rc func(int, int)
		rc = func(i, next int) {
			for j := next; j < n; j++ {
				s[i] = j
				if i == last {
					emit(s)
				} else {
					rc(i+1, j+1)
				}
			}
		}
		rc(0, 0)
	}

	combIndexes := func(n, m int) [][]int {
		res := [][]int{}
		for i := 1; i <= m; i++ {
			comb(n, i, func(c []int) {
				b := make([]int, len(c))
				copy(b, c)
				res = append(res, b)
			})
		}
		return res
	}

	indexes := combIndexes(len(arr), limit)
	res := make([][]Word, len(indexes))

	for i, v := range indexes {
		tmp := make([]Word, len(v))
		for j, w := range v {
			tmp[j] = arr[w]
		}
		res[i] = tmp
	}
	return res
}

func removeEditWithWordsByID(a *Alignment, ws ...Word) {
	toremove := []Edit{}

	for _, w := range ws {
		for _, v := range a.editMap {
			switch v := v.(type) {
			case *Ins:
				if v.W.ID == w.ID && v.W == w {
					toremove = append(toremove, v)
				}
			case *Del:
				if v.W.ID == w.ID && v.W == w {
					toremove = append(toremove, v)
				}
			case *Eq:
				if (v.From == w || v.To == w) && (v.From.ID == w.ID || v.To.ID == w.ID) {
					toremove = append(toremove, v)
				}
			case *Sub:
				for _, x := range append(v.From[:], v.To[:]...) {
					if x.ID == w.ID && x == w {
						toremove = append(toremove, v)
						break
					}
				}
			}
		}
	}
	a.remove(toremove...)
}
