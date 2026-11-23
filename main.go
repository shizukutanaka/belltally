// BellTally — a local-first Bellmark tally counter for PTA volunteers.
//
// One binary. It serves a small web UI on 127.0.0.1 only, keeps the
// tally in a single JSON file in the user's own data folder, and has
// no account, no cloud and no network client code. It only ever
// handles company numbers, point values and counts — never personal
// data.
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"belltally/internal/tally"
)

//go:embed web/index.html
var indexHTML []byte

const (
	version = "0.1.0"
	marker  = "BELLTALLY-APP"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			exit(serve())
			return
		case "summary":
			exit(cliSummary())
			return
		case "export":
			exit(cliExport(os.Args[2:]))
			return
		case "version", "-version", "--version":
			fmt.Println("belltally", version)
			return
		default:
			fmt.Fprintf(os.Stderr, "belltally: unknown command %q\n", os.Args[1])
			printUsage()
			os.Exit(1)
		}
	}
	exit(serve())
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: belltally [serve|summary|export|version]")
	fmt.Fprintln(os.Stderr, "  serve            start the web UI (default when no command is given)")
	fmt.Fprintln(os.Stderr, "  summary          print the current totals")
	fmt.Fprintln(os.Stderr, "  export csv [file]  write the tally out as CSV")
	fmt.Fprintln(os.Stderr, "  version          print the version")
}

func exit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "belltally:", err)
		os.Exit(1)
	}
}

// ---------- server ----------

type server struct {
	store *tally.Store
	mu    sync.Mutex
}

// stateView is what the UI renders: the raw entries, the aggregated
// summary, and the company master.
type stateView struct {
	Entries   []tally.Entry `json:"entries"`
	Companies []companyView `json:"companies"`
	Summary   summaryView   `json:"summary"`
}

type companyView struct {
	tally.Company
	Points []pointsView `json:"points,omitempty"`
}

type pointsView struct {
	Points string `json:"points"` // formatted, e.g. "0.5"
	Count  int    `json:"count"`
}

type summaryRow struct {
	Points   string `json:"points"`
	Count    int    `json:"count"`
	Subtotal string `json:"subtotal"`
}

type summaryCompany struct {
	Company     int          `json:"company"`
	Name        string       `json:"name"`
	Rows        []summaryRow `json:"rows"`
	TotalCount  int          `json:"total_count"`
	TotalPoints string       `json:"total_points"`
}

type summaryView struct {
	Companies   []summaryCompany `json:"companies"`
	TotalCount  int              `json:"total_count"`
	TotalPoints string           `json:"total_points"`
}

func buildState(b *tally.Book) stateView {
	sum := b.Summarize()
	sv := summaryView{TotalCount: sum.TotalCount, TotalPoints: tally.FormatPoints(sum.TotalPoints)}
	for _, d := range sum.Companies {
		sc := summaryCompany{Company: d.Company, Name: d.Name, TotalCount: d.TotalCount, TotalPoints: tally.FormatPoints(d.TotalPoints)}
		for _, r := range d.Rows {
			sc.Rows = append(sc.Rows, summaryRow{
				Points:   tally.FormatPoints(r.PointsHalf),
				Count:    r.Count,
				Subtotal: tally.FormatPoints(r.PointsHalf * r.Count),
			})
		}
		sv.Companies = append(sv.Companies, sc)
	}
	v := stateView{Summary: sv}
	v.Entries = b.Entries
	if v.Entries == nil {
		v.Entries = []tally.Entry{}
	}
	for _, c := range tally.Companies {
		cv := companyView{Company: c}
		canon := tally.Canonical(c.Number)
		for _, e := range b.Entries {
			if e.Company == canon {
				cv.Points = append(cv.Points, pointsView{Points: tally.FormatPoints(e.PointsHalf), Count: e.Count})
			}
		}
		v.Companies = append(v.Companies, cv)
	}
	return v
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, err.Error())
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		b, err := s.store.Load()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, buildState(b))
	})
	mux.HandleFunc("POST /api/count", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var req struct {
			Company int     `json:"company"`
			Points  float64 `json:"points"`
			Delta   int     `json:"delta"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeErr(w, 400, err)
			return
		}
		if req.Delta == 0 || req.Delta > 1000 || req.Delta < -1000 {
			writeErr(w, 400, errors.New("delta must be between -1000 and 1000, excluding 0"))
			return
		}
		b, err := s.store.Load()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		if err := b.Add(req.Company, req.Points, req.Delta); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := s.store.Save(b); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, buildState(b))
	})
	mux.HandleFunc("POST /api/clear", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		b := &tally.Book{}
		if err := s.store.Save(b); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, buildState(b))
	})
	mux.HandleFunc("GET /api/export.csv", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		b, err := s.store.Load()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"belltally.csv\"")
		_, _ = w.Write(tally.ExportCSV(b))
	})
	return mux
}

func isOurServer(port int) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return strings.Contains(string(body), marker)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func serve() error {
	store, err := tally.OpenStore()
	if err != nil {
		return err
	}
	s := &server{store: store}
	for port := 8781; port <= 8790; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			if isOurServer(port) {
				url := fmt.Sprintf("http://127.0.0.1:%d", port)
				fmt.Println("BellTally is already running at", url)
				openBrowser(url)
				return nil
			}
			continue // some other app owns this port; try the next one
		}
		url := fmt.Sprintf("http://127.0.0.1:%d", port)
		fmt.Println("BellTally", version, "—", url)
		fmt.Println("Data file:", store.Path)
		fmt.Println("Press Ctrl+C to stop.")
		openBrowser(url)
		return http.Serve(ln, s.routes())
	}
	return errors.New("no free port in 8781-8790")
}

// ---------- CLI ----------

func cliSummary() error {
	st, err := tally.OpenStore()
	if err != nil {
		return err
	}
	b, err := st.Load()
	if err != nil {
		return err
	}
	s := b.Summarize()
	for _, d := range s.Companies {
		fmt.Printf("%2d %-20s %5d 枚  %s 点\n", d.Company, d.Name, d.TotalCount, tally.FormatPoints(d.TotalPoints))
	}
	fmt.Printf("合計 %d 枚  %s 点\n", s.TotalCount, tally.FormatPoints(s.TotalPoints))
	return nil
}

func cliExport(args []string) error {
	if len(args) < 1 || args[0] != "csv" {
		return errors.New("usage: belltally export csv [file]")
	}
	st, err := tally.OpenStore()
	if err != nil {
		return err
	}
	b, err := st.Load()
	if err != nil {
		return err
	}
	raw := tally.ExportCSV(b)
	if len(args) > 1 {
		return os.WriteFile(args[1], raw, 0o644)
	}
	_, err = os.Stdout.Write(raw)
	return err
}
