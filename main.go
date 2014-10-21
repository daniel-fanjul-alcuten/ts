package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Curr struct {
	Text string
	Time time.Time
}
type Date struct {
	Year  int
	Month time.Month
	Day   int
}
type Past struct {
	Text     string
	Duration time.Duration
}
type Pasts []Past
type Hist struct {
	Date Date
	Past Pasts
}
type Hists []Hist
type Document struct {
	Curr Curr
	Hist Hists
}

func (d Date) Less(e Date) bool {
	if d.Year < e.Year {
		return true
	} else if d.Year > e.Year {
		return false
	}
	if d.Month < e.Month {
		return true
	} else if d.Month > e.Month {
		return false
	}
	if d.Day < e.Day {
		return true
	} else if d.Day > e.Day {
		return false
	}
	return false
}

func (p Pasts) Len() int           { return len(p) }
func (p Pasts) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p Pasts) Less(i, j int) bool { return p[i].Text < p[j].Text }

func (h Hists) Len() int           { return len(h) }
func (h Hists) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h Hists) Less(i, j int) bool { return h[i].Date.Less(h[j].Date) }

func (j *Document) ReadFrom(filename string) (err error) {
	var f *os.File
	if f, err = os.Open(filename); err != nil {
		if os.IsNotExist(err) {
			*j = Document{}
			err = nil
		}
		return
	}
	defer f.Close()
	r := bufio.NewReader(f)
	d := json.NewDecoder(r)
	if err = d.Decode(j); err != nil {
		if err != io.EOF {
			return
		}
		err = nil
	}
	if err = f.Close(); err != nil {
		return
	}
	return
}

func (j *Document) flush(now time.Time) {
	d := now.Sub(j.Curr.Time)
	if d <= 0 {
		j.Curr.Time = now
		return
	}
	p := Past{j.Curr.Text, d}
	var t Date
	t.Year, t.Month, t.Day = j.Curr.Time.Date()
	for i, h := range j.Hist {
		if h.Date == t {
			h.Past = append(h.Past, p)
			j.Hist[i] = h
			j.Curr.Time = now
			return
		}
	}
	j.Hist = append(j.Hist, Hist{t, []Past{p}})
	j.Curr.Time = now
	return
}

func (j *Document) Start(text string, now time.Time) {
	if !j.Curr.Time.IsZero() {
		j.flush(now)
	}
	if text != "" {
		j.Curr = Curr{text, now}
	}
	return
}

func (j *Document) Finish(now time.Time) {
	if j.Curr.Time.IsZero() {
		return
	}
	j.flush(now)
	j.Curr = Curr{}
}

func (j *Document) Discard(now time.Time) {
	if j.Curr.Time.IsZero() {
		return
	}
	j.Curr = Curr{}
}

func (j *Document) Clean() {
	if j.Curr.Time.IsZero() {
		j.Curr = Curr{}
	}
	sort.Sort(j.Hist)
	for i, h := range j.Hist {
		m := make(map[string]time.Duration)
		for _, p := range h.Past {
			m[p.Text] += p.Duration
		}
		h.Past = h.Past[:0]
		for t, d := range m {
			h.Past = append(h.Past, Past{t, d})
		}
		sort.Sort(h.Past)
		j.Hist[i] = h
	}
}

func (j *Document) Add(text string, d time.Duration, now time.Time) {
	if text == "" {
		return
	}
	p := Past{text, d}
	var t Date
	t.Year, t.Month, t.Day = now.Date()
	for i, h := range j.Hist {
		if h.Date == t {
			h.Past = append(h.Past, p)
			j.Hist[i] = h
			return
		}
	}
	j.Hist = append(j.Hist, Hist{t, []Past{p}})
	return
}

func (j *Document) Sub(sub time.Duration, now time.Time) {
	for sub > 0 {
		if len(j.Hist) == 0 {
			break
		}
		h := j.Hist[0]
		if len(h.Past) == 0 {
			j.Hist = j.Hist[1:]
			continue
		}
		p := h.Past[0]
		if p.Duration < sub {
			h.Past = h.Past[1:]
			j.Hist[0] = h
			// p.Duration -= p.Duration
			sub -= p.Duration
			continue
		}
		p.Duration -= sub
		sub = 0 // sub -= sub
		h.Past[0] = p
		j.Hist[0] = h
	}
	if sub > 0 {
		j.Add("sub", -sub, now)
	}
}

func (j *Document) Println(now time.Time) {
	for _, h := range j.Hist {
		fmt.Printf("%v/%v/%v\n", h.Date.Year, int(h.Date.Month), h.Date.Day)
		var d time.Duration
		for _, p := range h.Past {
			fmt.Printf("  %s: %v\n", p.Text, p.Duration)
			d += p.Duration
		}
		fmt.Printf("   %v\n", d)
	}
	if !j.Curr.Time.IsZero() {
		d := now.Sub(j.Curr.Time)
		if d == 0 {
			fmt.Printf("%s\n", j.Curr.Text)
		} else {
			fmt.Printf("%s for %v\n", j.Curr.Text, d)
		}
	}
}

func (j *Document) WriteTo(filename string) (err error) {
	var f *os.File
	if f, err = os.Create(filename); err != nil {
		return
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	e := json.NewEncoder(w)
	if err = e.Encode(j); err != nil {
		return
	}
	if err = w.Flush(); err != nil {
		return
	}
	if err = f.Close(); err != nil {
		return
	}
	return
}

func main() {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	ts := flag.String("ts", filepath.Join(u.HomeDir, ".ts.json"), "file")
	f := flag.Bool("f", false, "finish")
	d := flag.Bool("d", false, "discard")
	n := flag.Bool("n", false, "dry run")
	a := flag.Duration("a", 0, "add")
	s := flag.Duration("s", 0, "subtract")
	flag.Parse()
	var j Document
	if err = j.ReadFrom(*ts); err != nil {
		log.Fatal(err)
	}
	now := time.Now()
	text := strings.Join(flag.Args(), " ")
	if *n {
	} else if *f {
		j.Finish(now)
	} else if *d {
		j.Discard(now)
	} else if *a != 0 {
		j.Add(text, *a, now)
	} else if *s != 0 {
		j.Clean()
		j.Sub(*s, now)
	} else {
		j.Start(text, now)
	}
	j.Clean()
	if err = j.WriteTo(*ts); err != nil {
		log.Fatal(err)
	}
	j.Println(now)
}
