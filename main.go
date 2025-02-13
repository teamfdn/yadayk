package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/jtejido/sourceafis"
	"github.com/jtejido/sourceafis/config"
	"github.com/jtejido/sourceafis/templates"
)

type TransparencyContents struct{}

func (c *TransparencyContents) Accepts(key string) bool {
	return true
}

func (c *TransparencyContents) Accept(key, mime string, data []byte) error {
	return nil
}

func listFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	pool := sync.Pool{New: func() any { return new(string) }}

	for _, entry := range entries {
		if entry.IsDir() {
			entries, err := os.ReadDir(dir + "/" + entry.Name())
			if err != nil {
				return nil, err
			}
			for _, entry2 := range entries {
				if !entry2.IsDir() {
					fileName := pool.Get().(*string)
					*fileName = entry.Name() + "/" + entry2.Name()
					files = append(files, *fileName)
					pool.Put(fileName)
				}
			}
		} else {
			fileName := pool.Get().(*string)
			*fileName = entry.Name()
			files = append(files, *fileName)
			pool.Put(fileName)
		}
	}

	return files, nil
}

func preLoadTempls(files []string, tc *sourceafis.TemplateCreator, basePath string) map[string]*templates.SearchTemplate {
	templCache := make(map[string]*templates.SearchTemplate)
	for _, file := range files {
		img, err := sourceafis.LoadImage(basePath + "/" + file)
		if err != nil {
			log.Fatal(err)
		}

		templ, err := tc.Template(img)
		if err != nil {
			log.Fatal(err)
		}

		templCache[file] = templ
	}
	return templCache
}

func main() {
	var input string
	input = "sample-image"

	filesStart := time.Now()
	files, err := listFiles(input)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Raed done: ", time.Since(filesStart))

	config.LoadDefaultConfig()
	config.Config.Workers = runtime.NumCPU()
	probeImg, err := sourceafis.LoadImage("sample-image/1.png")
	if err != nil {
		log.Fatal(err.Error())
	}

	type kuadred struct {
		l     *sourceafis.DefaultTransparencyLogger
		tc    *sourceafis.TemplateCreator
		probe *templates.SearchTemplate
	}

	kuad := func(kk chan kuadred) {
		l := sourceafis.NewTransparencyLogger(new(TransparencyContents))
		tc := sourceafis.NewTemplateCreator(l)
		probe, err := tc.Template(probeImg)
		if err != nil {
			log.Fatal(err.Error())
		}
		kk <- kuadred{
			l:     l,
			tc:    tc,
			probe: probe,
		}
	}

	kkchan := make(chan kuadred)
	ff, _ := context.WithTimeout(context.TODO(), time.Millisecond*1000)

	go kuad(kkchan)

	var kuadreal *kuadred

CHECKIMG:
	for {
		select {
		case <-ff.Done():
			if errors.Is(ff.Err(), context.DeadlineExceeded) {
				panic("bobrok")
			}
		case kk := <-kkchan:
			kuadreal = &kk
			break CHECKIMG
		}
	}

	matcher, err := sourceafis.NewMatcher(kuadreal.l, kuadreal.probe)
	if err != nil {
		log.Fatal(err.Error())
	}

	templStart := time.Now()
	// chaching all template candidate
	templates := preLoadTempls(files, kuadreal.tc, input)
	fmt.Println("caching done:", templStart)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*1000)
	now := time.Now()
	res := make(chan struct {
		Name  string
		Score float64
	})

	for name, templ := range templates {
		if name == "1.png" {
			continue
		}
		go check(ctx, matcher, name, templ, res)
	}

loop:
	for {
		select {
		case <-ctx.Done():
			fmt.Println("done: ", time.Since(now))
			close(res)
			break loop

		case rr := <-res:
			fmt.Printf("score: %#+v \n", rr)
			if rr.Score >= 50 {
				fmt.Println("elapsed: ", time.Since(now))
				fmt.Printf("score: %#+v \n", rr)
				cancel()
			}
		}
	}

	fmt.Println("end")
}

func check(ctx context.Context, matcher *sourceafis.Matcher, fileName string, templ *templates.SearchTemplate, res chan<- struct {
	Name  string
	Score float64
},
) {
	var rr struct {
		Name  string
		Score float64
	}

	rr.Name = fileName
	rr.Score = matcher.Match(ctx, templ)

	select {
	case <-ctx.Done():
		fmt.Print("done")
		return
	default:
		res <- rr
	}
}
