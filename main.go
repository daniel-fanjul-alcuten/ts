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

type Document struct {
	Topic  string
	Since  time.Time
	Topics map[string]time.Duration
}

func (j *Document) Load(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			*j = Document{"", time.Time{}, map[string]time.Duration{}}
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
	if j.Topics == nil {
		j.Topics = map[string]time.Duration{}
	}
	return err
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
	return nil
}

func (j *Document) Flush(now time.Time) {
	if j.Topic == "" || j.Since.IsZero() {
		return
	}
	d := now.Sub(j.Since)
	if d <= 0 {
		return
	}
	j.Since = now
	j.Topics[j.Topic] += d
	return
}

func (j *Document) Println(now time.Time) {
	if len(j.Topics) > 0 {
		topics := make([]string, 0, len(j.Topics))
		for t := range j.Topics {
			topics = append(topics, t)
		}
		sort.Strings(topics)
		total := time.Duration(0)
		for _, t := range topics {
			d := j.Topics[t]
			fmt.Printf("%s: %v\n", t, d)
			total += d
		}
		fmt.Printf("  %v\n", total)
	}
	if j.Topic != "" && !j.Since.IsZero() {
		d := now.Sub(j.Since)
		if d == 0 {
			fmt.Printf("%s\n", j.Topic)
		} else {
			fmt.Printf("%s for %v\n", j.Topic, d)
		}
	}
}

var (
	Filename string
	Noop     bool
	Finish   bool
	Discard  bool
	Add      time.Duration
	Sub      time.Duration
	Update   bool
)

func init() {
	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	flag.StringVar(&Filename, "ts", filepath.Join(u.HomeDir, ".ts.json"), "The file name")
	flag.BoolVar(&Noop, "n", false, "Do Nothing")
	flag.BoolVar(&Finish, "f", false, "Finish")
	flag.BoolVar(&Discard, "d", false, "Discard")
	flag.DurationVar(&Add, "a", 0, "Add")
	flag.DurationVar(&Sub, "s", 0, "Subtract")
	flag.BoolVar(&Update, "u", false, "Update")
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
	now, topic := time.Now(), strings.Join(flag.Args(), " ")
	if Noop {
	} else if Finish {
		if topic != "" {
			j.Topic = topic
		}
		j.Flush(now)
		j.Topic, j.Since = "", time.Time{}
	} else if Discard {
		j.Topic, j.Since = "", time.Time{}
	} else if Add != 0 {
		if topic == "" {
			topic = j.Topic
		}
		if topic != "" {
			j.Topics[topic] += Add
		}
	} else if Sub != 0 {
		if topic == "" {
			topic = j.Topic
		}
		if topic != "" {
			j.Topics[topic] += -Sub
			j.Topics["_"] += j.Topics[topic]
			j.Topics[topic] -= j.Topics[topic]
		}
	} else if Update {
		if j.Topic != "" && topic != "" {
			j.Topic = topic
		}
		j.Flush(now)
		if topic != "" {
			j.Topic = topic
			j.Since = now
		}
	} else {
		j.Flush(now)
		if topic != "" {
			j.Topic = topic
			j.Since = now
		}
	}
	for t, d := range j.Topics {
		if d == 0 {
			delete(j.Topics, t)
		}
	}
	j.Println(now)
	if err := j.Save(Filename); err != nil {
		return err
	}
	return nil
}
