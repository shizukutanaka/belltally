package tally

// Company is one row of the built-in Bellmark sponsor master:
// the number printed inside the bell mark, and the company name.
type Company struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	// MergedInto is non-zero when this number is no longer tallied on
	// its own: marks carrying it are counted under the other number.
	MergedInto int    `json:"merged_into,omitempty"`
	Note       string `json:"note,omitempty"`
}

// Companies is the built-in master of current Bellmark sponsor
// companies (ベルマーク教育助成財団の協賛会社一覧に基づく、2026年11月時点).
// Numbers that are currently unassigned are simply absent; the UI
// still accepts any number so a stale master never blocks counting.
var Companies = []Company{
	{Number: 1, Name: "イオンリテール"},
	{Number: 3, Name: "ラッキーベル"},
	{Number: 4, Name: "ニッスイ"},
	{Number: 5, Name: "日本ノート"},
	{Number: 6, Name: "DM三井製糖"},
	{Number: 7, Name: "キユーピー"},
	{Number: 9, Name: "エスビー食品"},
	{Number: 10, Name: "クレハ"},
	{Number: 11, Name: "リネットジャパンリサイクル"},
	{Number: 15, Name: "ジブラルタ生命保険"},
	{Number: 17, Name: "日本テトラパック"},
	{Number: 18, Name: "日清オイリオグループ"},
	{Number: 19, Name: "キヤノンマーケティングジャパン"},
	{Number: 21, Name: "スリーエム ジャパン"},
	{Number: 24, Name: "日清製粉ウェルナ"},
	{Number: 25, Name: "明治"},
	{Number: 26, Name: "森永製菓"},
	{Number: 28, Name: "ブラザー販売"},
	{Number: 31, Name: "新学社"},
	{Number: 33, Name: "ヤマハミュージックジャパン"},
	{Number: 35, Name: "日清食品"},
	{Number: 39, Name: "ファインプラス"},
	{Number: 43, Name: "東芝ライフスタイル"},
	{Number: 48, Name: "ブルボン"},
	{Number: 52, Name: "ナカバヤシ"},
	{Number: 53, Name: "ショウワノート"},
	{Number: 54, Name: "キリンビバレッジ"},
	{Number: 55, Name: "クツワ"},
	{Number: 64, Name: "マルトモ"},
	{Number: 70, Name: "スミフル"},
	{Number: 73, Name: "エプソン販売"},
	{Number: 76, Name: "NGP日本自動車リサイクル事業協同組合"},
	{Number: 77, Name: "湖池屋"},
	{Number: 78, Name: "グンゼ"},
	{Number: 84, Name: "セメダイン"},
	{Number: 88, Name: "ニチレイ"},
	{Number: 89, Name: "白鶴酒造"},
	{Number: 91, Name: "トヤマ楽器製造"},
	{Number: 92, Name: "あいおいニッセイ同和損害保険"},
	{Number: 93, Name: "石井食品"},
	// Numbers retired by the foundation's number consolidation
	// (番号統合). Marks with these numbers still circulate, and the
	// foundation's tally guidance counts them under the surviving
	// number, so they stay selectable and are folded in at tally time.
	{Number: 20, Name: "スリーエム ジャパン", MergedInto: 21, Note: "2019年から21番に統合"},
	{Number: 46, Name: "ニッスイ", MergedInto: 4, Note: "2018年から4番に統合"},
}

var byNumber = func() map[int]Company {
	m := make(map[int]Company, len(Companies))
	for _, c := range Companies {
		m[c.Number] = c
	}
	return m
}()

// Lookup returns the master row for a number, if any.
func Lookup(number int) (Company, bool) {
	c, ok := byNumber[number]
	return c, ok
}

// Canonical folds a number through the consolidation rules:
// 20 -> 21 and 46 -> 4. Every other number is returned unchanged.
func Canonical(number int) int {
	if c, ok := byNumber[number]; ok && c.MergedInto != 0 {
		return c.MergedInto
	}
	return number
}

// Name returns the display name for a (canonical) number, or a
// placeholder when the number is not in the built-in master.
func Name(number int) string {
	if c, ok := byNumber[number]; ok {
		return c.Name
	}
	return "（マスタ未登録）"
}
