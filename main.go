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

type Past struct {
	Text     string
	Duration time.Duration
}

type Pasts []Past

func (p Pasts) Len() int {
	return len(p)
}

func (p Pasts) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p Pasts) Less(i, j int) bool {
	return p[i].Text < p[j].Text
}

type Hist struct {
	Date Date
	Past Pasts
}

type Hists []Hist

func (h Hists) Len() int {
	return len(h)
}

func (h Hists) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h Hists) Less(i, j int) bool {
	return h[i].Date.Less(h[j].Date)
}

type Document struct {
	Curr Curr
	Hist Hists
}

func (j *Document) Load(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			*j = Document{}
			err = nil
		}
		return err
	}
	defer f.Close()
	d := json.NewDecoder(bufio.NewReader(f))
	err = d.Decode(j)
	if err == io.EOF {
		err = nil
	}
	return err
}

func (j *Document) Flush(now time.Time) {
	if j.Curr.Text == "" || j.Curr.Time.IsZero() {
		return
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

func (j *Document) Add(d time.Duration, text string, now time.Time) {
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

func (j *Document) Println(now time.Time) {
	for _, h := range j.Hist {
		fmt.Printf("%v/%v/%v\n", h.Date.Year, int(h.Date.Month), h.Date.Day)
		var d time.Duration
		for _, p := range h.Past {
			fmt.Printf("  %s: %v\n", p.Text, p.Duration)
			d += p.Duration
		}
		fmt.Printf("    %v\n", d)
	}
	if j.Curr.Text != "" && !j.Curr.Time.IsZero() {
		d := now.Sub(j.Curr.Time)
		if d == 0 {
			fmt.Printf("%s\n", j.Curr.Text)
		} else {
			fmt.Printf("%s for %v\n", j.Curr.Text, d)
		}
	}
}

func (j *Document) Save(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	e := json.NewEncoder(w)
	if err := e.Encode(j); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

var (
	Filename string
	Finish   bool
	Discard  bool
	Noop     bool
	Add      time.Duration
	Sub      time.Duration
)

func init() {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	flag.StringVar(&Filename, "ts", filepath.Join(u.HomeDir, ".ts.json"), "file")
	flag.BoolVar(&Noop, "n", false, "noop")
	flag.BoolVar(&Finish, "f", false, "finish")
	flag.BoolVar(&Discard, "d", false, "discard")
	flag.DurationVar(&Add, "a", 0, "add")
	flag.DurationVar(&Sub, "s", 0, "subtract")
}

func main() {
	flag.Parse()
	if err := do(); err != nil {
		log.Fatal(err)
	}
}

func do() error {
	var j Document
	if err := j.Load(Filename); err != nil {
		return err
	}
	now, text := time.Now(), strings.Join(flag.Args(), " ")
	if Noop {
	} else if Finish {
		if text != "" {
			j.Curr.Text = text
		}
		j.Flush(now)
		j.Curr = Curr{}
	} else if Discard {
		j.Curr = Curr{}
	} else if Add != 0 {
		j.Add(Add, text, now)
	} else if Sub != 0 {
		j.Add(-Sub, text, now)
	} else {
		j.Flush(now)
		if text != "" {
			j.Curr.Text = text
		}
		if j.Curr.Text != "" {
			j.Curr.Time = now
		}
	}
	j.Clean()
	if err := j.Save(Filename); err != nil {
		return err
	}
	j.Println(now)
	return nil
}
