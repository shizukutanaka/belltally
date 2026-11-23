package tally

import (
	"bytes"
	"encoding/csv"
	"strconv"
)

// ExportCSV renders the tally as one CSV file with two sections: the
// per-company, per-point-value detail (for transcribing onto the
// sorting bags), then the per-company totals and the grand total
// (for the shipping form). A UTF-8 BOM is prepended so spreadsheet
// apps open it with the Japanese text intact.
func ExportCSV(b *Book) []byte {
	s := b.Summarize()
	var buf bytes.Buffer
	buf.WriteString("\ufeff")
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"企業番号", "会社名", "点数", "枚数", "点数小計"})
	for _, d := range s.Companies {
		for _, r := range d.Rows {
			_ = w.Write([]string{
				strconv.Itoa(d.Company),
				d.Name,
				FormatPoints(r.PointsHalf),
				strconv.Itoa(r.Count),
				FormatPoints(r.PointsHalf * r.Count),
			})
		}
	}
	_ = w.Write([]string{})
	_ = w.Write([]string{"会社別合計"})
	_ = w.Write([]string{"企業番号", "会社名", "合計枚数", "合計点数"})
	for _, d := range s.Companies {
		_ = w.Write([]string{
			strconv.Itoa(d.Company),
			d.Name,
			strconv.Itoa(d.TotalCount),
			FormatPoints(d.TotalPoints),
		})
	}
	_ = w.Write([]string{"", "総合計", strconv.Itoa(s.TotalCount), FormatPoints(s.TotalPoints)})
	w.Flush()
	return buf.Bytes()
}
