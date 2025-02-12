package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/jtejido/sourceafis"
	"github.com/jtejido/sourceafis/config"
)

type TransparencyContents struct{}

func (c *TransparencyContents) Accepts(key string) bool {
	return true
}

func (c *TransparencyContents) Accept(key, mime string, data []byte) error {
	// fmt.Printf("%d B  %s %s \n", len(data), mime, key)
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
		if !entry.IsDir() {
			fileName := pool.Get().(*string)
			*fileName = entry.Name()
			files = append(files, *fileName)
			pool.Put(fileName)
		}
	}

	return files, nil
}

func main() {
	var input string
	fmt.Print("Masukan directory: ")
	fmt.Scanln(&input)

	input = "sample-image"
	files, err := listFiles(input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("============================")

	config.LoadDefaultConfig()
	config.Config.Workers = runtime.NumCPU()
	probeImg, err := sourceafis.LoadImage(input + "/1.png")
	if err != nil {
		log.Fatal(err.Error())
	}
	l := sourceafis.NewTransparencyLogger(new(TransparencyContents))
	tc := sourceafis.NewTemplateCreator(l)
	probe, err := tc.Template(probeImg)
	if err != nil {
		log.Fatal(err.Error())
	}

	matcher, err := sourceafis.NewMatcher(l, probe)
	if err != nil {
		log.Fatal(err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	now := time.Now()
	res := make(chan struct {
		Name  string
		Score float64
	})

	for _, file := range files {
		go check(ctx, tc, matcher, input+"/"+file, res)
	}

loop:
	for {
		select {
		case <-ctx.Done():
			fmt.Println("done: ", time.Since(now))
			close(res)
			break loop

		case rr := <-res:
			if rr.Score >= 50 {
				fmt.Println("elapsed: ", time.Since(now))
				fmt.Printf("score: %#+v \n", rr)
				cancel()
			}
		}
	}

	fmt.Println("end")
}

func check(ctx context.Context, tc *sourceafis.TemplateCreator, matcher *sourceafis.Matcher, file string, res chan<- struct {
	Name  string
	Score float64
},
) {
	var rr struct {
		Name  string
		Score float64
	}
	img, err := sourceafis.LoadImage(file)
	if err != nil {
		return
	}

	candidate, err := tc.Template(img)
	if err != nil {
		return
	}
	rr.Name = file
	rr.Score = matcher.Match(ctx, candidate)

	select {
	case <-ctx.Done():
		fmt.Print("done")
		return
	default:
		res <- rr
	}
}
