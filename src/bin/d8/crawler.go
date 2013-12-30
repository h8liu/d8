package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"d8/client"
	. "d8/domain"
	"d8/tasks"
	"d8/term"
	"printer"
)

type Crawler struct {
	In      string
	Out     string
	Quota   int
	Log     io.Writer
	Deflate bool
}

type crawlTask struct {
	id      string
	domain  *Domain
	client  *client.Client
	out     *zip.Writer
	deflate bool
	lock    *sync.Mutex

	quota       int
	quotaReturn chan int
}

func (self *Crawler) quotas() chan int {
	nquota := self.Quota
	if nquota == 0 {
		nquota = 1
	}
	ret := make(chan int, nquota)
	for i := 0; i < nquota; i++ {
		ret <- i
	}

	return ret
}

func digits(a int) int {
	if a <= 9 {
		return 1
	}
	ret := 1
	for a >= 10 {
		a /= 10
		ret++
	}
	return ret
}

func (self *Crawler) Crawl() error {
	// load input
	list, e := LoadList(self.In, self.Log)
	if e != nil {
		return e
	}

	// init output
	fout, e := os.Create(self.Out)
	if e != nil {
		return e
	}
	out := zip.NewWriter(fout)
	defer out.Close()

	c, e := client.New()
	if e != nil {
		return e
	}

	quotas := self.quotas()
	lock := new(sync.Mutex)

	idFmt := fmt.Sprintf("%%0%dd", digits(len(list)))

	for id, d := range list {
		q := <-quotas
		task := &crawlTask{
			id:          fmt.Sprintf(idFmt, id+1),
			domain:      d,
			client:      c,
			out:         out,
			deflate:     self.Deflate,
			lock:        lock,
			quota:       q,
			quotaReturn: quotas,
		}

		go task.run()
	}

	// join
	for i := 0; i < cap(quotas); i++ {
		<-quotas
	}

	return nil
}

func (self *crawlTask) create(path string) (io.Writer, error) {
	header := &zip.FileHeader{Name: path}
	if self.deflate {
		header.Method = zip.Deflate
	}

	return self.out.CreateHeader(header)
}

func (self *crawlTask) path(dir string) string {
	s := self.domain.String()
	if len(s) > 200 {
		s = s[:200]
	}
	if len(s) == 0 {
		s = "."
	}

	return fmt.Sprintf("%s/%s_%s", dir, self.id, s)
}

func (self *crawlTask) run() {
	logbuf := new(bytes.Buffer)
	t := term.New(self.client)
	t.Log = logbuf
	info := tasks.NewInfo(self.domain)
	_, err := t.T(info)

	self.lock.Lock()

	fout, e := self.create(self.path("log"))
	noError(e)
	_, e = io.Copy(fout, logbuf)
	noError(e)

	fout, e = self.create(self.path("out"))
	if err == nil {
		e = printInfo(info, fout)
		noError(e)
	} else {
		fmt.Fprintf(fout, "error: %v\n", err)
	}

	self.lock.Unlock()

	self.quotaReturn <- self.quota
}

func printInfo(info *tasks.Info, out io.Writer) error {
	p := printer.New(out)
	p.Printf("// %s", info.Domain)
	p.Println()

	info.PrintTo(p)

	return p.Error
}