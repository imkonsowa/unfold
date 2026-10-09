package configured

type pair struct{ A, B int }

var one = []int{1}

var two = []int{1, 2} // want `composite literal is inlined`
