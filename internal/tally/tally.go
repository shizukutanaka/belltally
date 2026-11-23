package tally

import (
	"errors"
	"fmt"
	"sort"
)

// Points are handled as integer half-points (points x 2) everywhere
// inside the tally, so sums of 0.5-step values are always exact and
// never accumulate floating point error.

// ParsePoints converts a point value given as a float (e.g. from JSON)
// into half-points. Only positive multiples of 0.5 are valid Bellmark
// point values.
func ParsePoints(points float64) (int, error) {
	half := points * 2
	rounded := int(half + 0.5)
	if points <= 0 || float64(rounded) != half {
		return 0, fmt.Errorf("点数は0.5点刻みの正の値で指定してください: %v", points)
	}
	return rounded, nil
}

// FormatPoints renders half-points the way they are printed on the
// marks: whole numbers without a fraction, halves as "x.5".
func FormatPoints(half int) string {
	if half%2 == 0 {
		return fmt.Sprintf("%d", half/2)
	}
	return fmt.Sprintf("%d.5", half/2)
}

// Entry is one counted pile: how many marks of one company carry one
// point value. Company is always stored canonicalised (see Canonical),
// PointsHalf is the point value times two.
type Entry struct {
	Company    int `json:"company"`
	PointsHalf int `json:"points_half"`
	Count      int `json:"count"`
}

// Book is the whole tally: a set of entries keyed by
// (company, point value).
type Book struct {
	Entries []Entry `json:"entries"`
}

func (b *Book) index(company, pointsHalf int) int {
	for i, e := range b.Entries {
		if e.Company == company && e.PointsHalf == pointsHalf {
			return i
		}
	}
	return -1
}

// Add applies a delta (+1, +10, -1, ...) to one pile. The company
// number is folded through the consolidation rules first, so counting
// a 20 mark lands on 21. A pile never goes below zero.
func (b *Book) Add(company int, points float64, delta int) error {
	half, err := ParsePoints(points)
	if err != nil {
		return err
	}
	if company < 1 || company > 99 {
		return errors.New("企業番号は1〜99で指定してください")
	}
	company = Canonical(company)
	i := b.index(company, half)
	cur := 0
	if i >= 0 {
		cur = b.Entries[i].Count
	}
	if cur+delta < 0 {
		return fmt.Errorf("枚数がマイナスになります（現在 %d 枚）", cur)
	}
	switch {
	case i < 0 && delta > 0:
		b.Entries = append(b.Entries, Entry{Company: company, PointsHalf: half, Count: delta})
	case i >= 0 && cur+delta == 0:
		b.Entries = append(b.Entries[:i], b.Entries[i+1:]...)
	case i >= 0:
		b.Entries[i].Count = cur + delta
	}
	return nil
}

// CountOf returns the current count of one pile (0 when absent).
func (b *Book) CountOf(company int, points float64) int {
	half, err := ParsePoints(points)
	if err != nil {
		return 0
	}
	if i := b.index(Canonical(company), half); i >= 0 {
		return b.Entries[i].Count
	}
	return 0
}

// Clear empties the book.
func (b *Book) Clear() { b.Entries = nil }

// Detail is one company's breakdown: one row per point value, plus
// the company's totals. Points are in half-points.
type Detail struct {
	Company     int
	Name        string
	Rows        []Entry // sorted by point value
	TotalCount  int
	TotalPoints int // half-points
}

// Summary is the full aggregation: per-company details sorted by
// company number, and the grand totals (points in half-points).
type Summary struct {
	Companies   []Detail
	TotalCount  int
	TotalPoints int
}

// Summarize aggregates the book. Entries are re-canonicalised here as
// well, so even a hand-edited data file tallies by the official rules.
func (b *Book) Summarize() Summary {
	merged := map[int]map[int]int{} // company -> pointsHalf -> count
	for _, e := range b.Entries {
		c := Canonical(e.Company)
		if merged[c] == nil {
			merged[c] = map[int]int{}
		}
		merged[c][e.PointsHalf] += e.Count
	}
	var s Summary
	for company, piles := range merged {
		d := Detail{Company: company, Name: Name(company)}
		points := make([]int, 0, len(piles))
		for p := range piles {
			points = append(points, p)
		}
		sort.Ints(points)
		for _, p := range points {
			n := piles[p]
			if n == 0 {
				continue
			}
			d.Rows = append(d.Rows, Entry{Company: company, PointsHalf: p, Count: n})
			d.TotalCount += n
			d.TotalPoints += p * n
		}
		if d.TotalCount == 0 {
			continue
		}
		s.Companies = append(s.Companies, d)
		s.TotalCount += d.TotalCount
		s.TotalPoints += d.TotalPoints
	}
	sort.Slice(s.Companies, func(i, j int) bool { return s.Companies[i].Company < s.Companies[j].Company })
	return s
}
