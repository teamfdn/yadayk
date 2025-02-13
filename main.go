package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
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
	// fmt.Printf("%d B  %s %s \n", len(data), mime, key)
	return nil
}

func listFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		path := entry.Name()
		if entry.IsDir() {
			subEntries, err := os.ReadDir(path)
			if err != nil {
				continue // Skip unreadable directories instead of returning an error
			}
			for _, subEntry := range subEntries {
				if !subEntry.IsDir() {
					files = append(files, path+"/"+subEntry.Name())
				}
			}
		} else {
			files = append(files, path)
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

	files, err := listFiles(input)
	if err != nil {
		log.Fatal(err)
	}

	config.LoadDefaultConfig()
	config.Config.Workers = runtime.NumCPU()
	probeImg, err := sourceafis.LoadImage("sample-image/1.png")
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

	// chaching all template candidate
	templates := preLoadTempls(files, tc, input)

	// dynamic timout
	timeout := time.Duration(len(templates)) * time.Millisecond * 500

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	now := time.Now()
	res := make(chan struct {
		Name  string
		Score float64
	}, len(templates))

	for name, templ := range templates {
		if name == "i.png" {
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
