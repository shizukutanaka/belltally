package tally

import (
	"strings"
	"testing"
)

func TestCanonicalMerges(t *testing.T) {
	cases := map[int]int{
		20: 21, // スリーエム ジャパン: 2019年から21番に統合
		46: 4,  // ニッスイ: 2018年から4番に統合
		21: 21,
		4:  4,
		7:  7,
		99: 99, // unknown numbers pass through untouched
	}
	for in, want := range cases {
		if got := Canonical(in); got != want {
			t.Errorf("Canonical(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestMasterHasMergedRows(t *testing.T) {
	c, ok := Lookup(20)
	if !ok || c.MergedInto != 21 {
		t.Errorf("Lookup(20) = %+v, %v; want MergedInto 21", c, ok)
	}
	c, ok = Lookup(46)
	if !ok || c.MergedInto != 4 {
		t.Errorf("Lookup(46) = %+v, %v; want MergedInto 4", c, ok)
	}
	if Name(4) != "ニッスイ" || Name(21) != "スリーエム ジャパン" {
		t.Errorf("unexpected names: 4=%q 21=%q", Name(4), Name(21))
	}
}

func TestAddFoldsMergedNumbers(t *testing.T) {
	var b Book
	if err := b.Add(20, 1, 3); err != nil {
		t.Fatal(err)
	}
	if err := b.Add(21, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := b.Add(46, 2, 1); err != nil {
		t.Fatal(err)
	}
	if got := b.CountOf(21, 1); got != 5 {
		t.Errorf("CountOf(21, 1) = %d, want 5 (20 folded into 21)", got)
	}
	if got := b.CountOf(20, 1); got != 5 {
		t.Errorf("CountOf(20, 1) = %d, want 5 (lookup also folds)", got)
	}
	if got := b.CountOf(4, 2); got != 1 {
		t.Errorf("CountOf(4, 2) = %d, want 1 (46 folded into 4)", got)
	}
	s := b.Summarize()
	if len(s.Companies) != 2 {
		t.Fatalf("Summarize companies = %d, want 2 (no separate 20/46 rows)", len(s.Companies))
	}
}

func TestHalfPointSumsAreExact(t *testing.T) {
	var b Book
	// 0.5-point marks are where float arithmetic would drift; the
	// tally must stay exact.
	for i := 0; i < 7; i++ {
		if err := b.Add(7, 0.5, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Add(7, 1.5, 3); err != nil {
		t.Fatal(err)
	}
	if err := b.Add(7, 2.5, 2); err != nil {
		t.Fatal(err)
	}
	s := b.Summarize()
	// 7*0.5 + 3*1.5 + 2*2.5 = 3.5 + 4.5 + 5 = 13 points = 26 half.
	if s.TotalPoints != 26 {
		t.Errorf("TotalPoints = %d half, want 26", s.TotalPoints)
	}
	if got := FormatPoints(s.TotalPoints); got != "13" {
		t.Errorf("FormatPoints(total) = %q, want %q", got, "13")
	}
	if s.TotalCount != 12 {
		t.Errorf("TotalCount = %d, want 12", s.TotalCount)
	}
}

func TestFormatPoints(t *testing.T) {
	cases := map[int]string{0: "0", 1: "0.5", 2: "1", 3: "1.5", 10: "5", 25: "12.5", 100: "50"}
	for half, want := range cases {
		if got := FormatPoints(half); got != want {
			t.Errorf("FormatPoints(%d) = %q, want %q", half, got, want)
		}
	}
}

func TestParsePointsRejectsNonHalfSteps(t *testing.T) {
	for _, bad := range []float64{0, -1, 0.3, 1.25} {
		if _, err := ParsePoints(bad); err == nil {
			t.Errorf("ParsePoints(%v) succeeded, want error", bad)
		}
	}
	for _, good := range []float64{0.5, 1, 2.5, 50} {
		if _, err := ParsePoints(good); err != nil {
			t.Errorf("ParsePoints(%v) error: %v", good, err)
		}
	}
}

func TestCountNeverGoesNegative(t *testing.T) {
	var b Book
	if err := b.Add(9, 1, -1); err == nil {
		t.Error("Add(-1) on empty pile succeeded, want error")
	}
	if err := b.Add(9, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := b.Add(9, 1, -3); err == nil {
		t.Error("Add(-3) below zero succeeded, want error")
	}
	if err := b.Add(9, 1, -2); err != nil {
		t.Fatal(err)
	}
	if got := b.CountOf(9, 1); got != 0 {
		t.Errorf("CountOf after draining = %d, want 0", got)
	}
	if s := b.Summarize(); s.TotalCount != 0 || len(s.Companies) != 0 {
		t.Errorf("drained book summarized to %+v, want empty", s)
	}
}

func TestSummarizeSortsAndTotals(t *testing.T) {
	var b Book
	_ = b.Add(35, 5, 2)
	_ = b.Add(7, 1, 10)
	_ = b.Add(35, 1, 1)
	_ = b.Add(35, 5, 1)
	s := b.Summarize()
	if len(s.Companies) != 2 || s.Companies[0].Company != 7 || s.Companies[1].Company != 35 {
		t.Fatalf("companies not sorted by number: %+v", s.Companies)
	}
	d := s.Companies[1]
	if len(d.Rows) != 2 || d.Rows[0].PointsHalf != 2 || d.Rows[1].PointsHalf != 10 {
		t.Errorf("rows not sorted by points: %+v", d.Rows)
	}
	if d.TotalCount != 4 || d.TotalPoints != 2*1+10*3 {
		t.Errorf("company totals = %d枚 %d half, want 4枚 %d half", d.TotalCount, d.TotalPoints, 2+30)
	}
	if s.TotalCount != 14 {
		t.Errorf("grand TotalCount = %d, want 14", s.TotalCount)
	}
}

func TestExportCSVFormat(t *testing.T) {
	var b Book
	_ = b.Add(20, 1, 2) // folds to 21
	_ = b.Add(4, 0.5, 4)
	_ = b.Add(4, 1, 1)
	out := string(ExportCSV(&b))
	if !strings.HasPrefix(out, "\ufeff") {
		t.Error("CSV lacks UTF-8 BOM")
	}
	lines := strings.Split(strings.TrimRight(strings.TrimPrefix(out, "\ufeff"), "\r\n"), "\n")
	trim := func(s string) string { return strings.TrimRight(s, "\r") }
	if got := trim(lines[0]); got != "企業番号,会社名,点数,枚数,点数小計" {
		t.Errorf("header = %q", got)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"4,ニッスイ,0.5,4,2",      // detail: 4 x 0.5 = 2
		"4,ニッスイ,1,1,1",        // detail
		"21,スリーエム ジャパン,1,2,2", // 20 folded into 21 in the detail too
		"会社別合計",
		"4,ニッスイ,5,3",        // company total: 5枚, 3点
		"21,スリーエム ジャパン,2,2", // company total
		",総合計,7,5",          // grand total: 7枚, 5点
	} {
		found := false
		for _, ln := range lines {
			if trim(ln) == want {
				found = true
			}
		}
		if !found {
			t.Errorf("CSV missing line %q\n---\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "46,") || strings.Contains(joined, "\n20,") {
		t.Errorf("CSV contains unmerged 20/46 rows:\n%s", joined)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	st := &Store{Path: t.TempDir() + "/belltally.json"}
	b, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Add(46, 3, 5); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(b); err != nil {
		t.Fatal(err)
	}
	back, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := back.CountOf(4, 3); got != 5 {
		t.Errorf("after round trip CountOf(4, 3) = %d, want 5", got)
	}
}
