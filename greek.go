package main

import (
	"reflect"
	"sort"
	"sync"
)

type greekAligner struct{}

func NewGreekAligner() *greekAligner {
	return &greekAligner{}
}

func (*greekAligner) next(a *alignment, subSeqLen int) []alignment {
	nextAlignments := []alignment{}

	dels := a.filter(reflect.TypeOf(&del{}))
	if len(dels) == 0 {
		return nextAlignments
	}
	inss := a.filter(reflect.TypeOf(&ins{}))

	// Add Eqs
	for _, x := range dels {
		x := x.(*del)
		for _, y := range inss {
			y := y.(*ins)
			if x.w.text != y.w.text {
				continue
			}
			eq := eq{
				from: x.w,
				to:   y.w,
			}
			newAlign := a.clone()
			newAlign.remove(x, y)
			newAlign.add(&eq)
			nextAlignments = append(nextAlignments, newAlign)
		}
	}

	if len(nextAlignments) > 0 {
		return nextAlignments
	}

	sort.SliceStable(dels, func(x, y int) bool { return dels[x].(*del).w.ID < dels[y].(*del).w.ID })
	sort.SliceStable(inss, func(x, y int) bool { return inss[x].(*ins).w.ID < inss[y].(*ins).w.ID })

	delWords := []word{}
	for _, x := range dels {
		delWords = append(delWords, x.(*del).w)
	}
	subDels := limitedSubsequences(delWords, subSeqLen)

	insWords := []word{}
	for _, x := range inss {
		insWords = append(insWords, x.(*ins).w)
	}
	subIns := limitedSubsequences(insWords, subSeqLen)

	for _, d := range subDels {
		for _, i := range subIns {
			newSubEdit := sub{
				from: d,
				to:   i,
			}
			newAlignment := a.clone()
			removeEditWithWordsByID(&newAlignment, append(d, i...)...)
			newAlignment.add(&newSubEdit)

			nextAlignments = append(nextAlignments, newAlignment)
		}
	}
	return nextAlignments
}

// func removeEditWithWords(a *alignment, ws ...word) {
// 	toremove := []edit{}

// 	inss := a.filter(reflect.TypeOf(&ins{}))
// 	dels := a.filter(reflect.TypeOf(&del{}))
// 	eqs := a.filter(reflect.TypeOf(&eq{}))
// 	subs := a.filter(reflect.TypeOf(&sub{}))

// 	for _, w := range ws {
// 		for _, v := range inss {
// 			if v.(*ins).w == w {
// 				toremove = append(toremove, v)
// 			}
// 		}
// 		for _, v := range dels {
// 			if v.(*del).w == w {
// 				toremove = append(toremove, v)
// 			}
// 		}
// 		for _, v := range eqs {
// 			if v.(*q).Source == w || v.(*wordsaligner.Eq).Target == w {
// 				toremove = append(toremove, v)
// 			}
// 		}
// 		for _, v := range subs {
// 			for _, x := range append(v.(*Sub).Source[:], v.(*Sub).Target[:]...) {
// 				if x == w {
// 					toremove = append(toremove, v)
// 					break
// 				}
// 			}
// 		}
// 	}
// 	a.Remove(toremove...)
// }

// Scholie data
type Scholie map[string]map[string][]string

var mutex = &sync.Mutex{}

func limitedSubsequences(arr []word, limit int) [][]word {
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
			return
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
	res := make([][]word, len(indexes))

	for i, v := range indexes {
		tmp := make([]word, len(v))
		for j, w := range v {
			tmp[j] = arr[w]
		}
		res[i] = tmp
	}
	return res
}

func removeEditWithWordsByID(a *alignment, ws ...word) {
	toremove := []edit{}

	for _, w := range ws {
		for _, v := range a.editMap {
			switch v.(type) {
			case *ins:
				if v.(*ins).w.ID == w.ID && v.(*ins).w == w {
					toremove = append(toremove, v)
				}
			case *del:
				if v.(*del).w.ID == w.ID && v.(*del).w == w {
					toremove = append(toremove, v)
				}
			case *eq:
				if (v.(*eq).from == w || v.(*eq).to == w) && (v.(*eq).from.ID == w.ID || v.(*eq).to.ID == w.ID) {
					toremove = append(toremove, v)
				}
			case *sub:
				for _, x := range append(v.(*sub).from[:], v.(*sub).to[:]...) {
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
