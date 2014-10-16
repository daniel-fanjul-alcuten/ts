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

func (j *Document) Start(text string, now time.Time) {
	if j.Curr.Time.IsZero() {
		j.Curr = Curr{text, now}
		return
	}
	if text == "" {
		return
	}
	if j.Curr.Text == text {
		return
	}
	if j.Curr.Text == "" {
		j.Curr.Text = text
		return
	}
	j.Finish(j.Curr.Text, now)
	j.Curr = Curr{text, now}
	return
}

func (j *Document) Finish(text string, now time.Time) {
	defer func() {
		j.Curr = Curr{}
	}()
	if j.Curr.Time.IsZero() {
		return
	}
	if text != "" {
		j.Curr.Text = text
	}
	d := now.Sub(j.Curr.Time)
	if d <= 0 {
		return
	}
	p := Past{j.Curr.Text, d}
	var t Date
	t.Year, t.Month, t.Day = j.Curr.Time.Date()
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

func (j *Document) Println(now time.Time) {
	for _, h := range j.Hist {
		var d time.Duration
		for _, p := range h.Past {
			d += p.Duration
		}
		fmt.Printf("%v/%v/%v: %v\n", h.Date.Year, int(h.Date.Month), h.Date.Day, d)
		for _, p := range h.Past {
			fmt.Printf("  %v: %s\n", p.Duration, p.Text)
		}
	}
	if !j.Curr.Time.IsZero() {
		fmt.Printf("%v: %s\n", now.Sub(j.Curr.Time), j.Curr.Text)
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
	d := flag.String("data", filepath.Join(u.HomeDir, ".ts.json"), "data file")
	f := flag.Bool("f", false, "finish")
	flag.Parse()
	var j Document
	if err = j.ReadFrom(*d); err != nil {
		log.Fatal(err)
	}
	now := time.Now()
	text := strings.Join(flag.Args(), " ")
	if *f {
		j.Finish(text, now)
	} else {
		j.Start(text, now)
	}
	j.Clean()
	if err = j.WriteTo(*d); err != nil {
		log.Fatal(err)
	}
	j.Println(now)
}
