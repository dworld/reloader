package main

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/ActiveState/tail"
	"github.com/romanoff/fsmonitor"
	"gopkg.in/yaml.v1"
)

const (
	config = "Reloader.yaml"
)

var (
	fileSummarys = map[string]string{}
	runMutex     sync.Mutex
)

type Config struct {
	Watch []struct {
		Pattern string
		Command string
		Log     string
		Delay   int
		Start   int
	}
	Skip struct {
		Folders []string
	}
}

func md5String(filename string) string {
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Println(err)
		return ""
	}
	a := md5.Sum(bs)
	return string(a[:])
}

func changed(name string) (b bool) {
	oldSummary := fileSummarys[name]
	newSummary := md5String(name)
	if oldSummary == "" {
		fileSummarys[name] = newSummary
		return true
	}
	if oldSummary == newSummary {
		return false
	} else {
		fileSummarys[name] = newSummary
		return true
	}
}

func printOpenFileLimit() {
	rlimit := &syscall.Rlimit{}
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlimit)
	log.Printf("open file limit, %v", rlimit)
}

func main() {
	f, err := ioutil.ReadFile(config)
	if err != nil {
		panic(err)
	}
	c := Config{}
	if err := yaml.Unmarshal(f, &c); err != nil {
		panic(err)
	}
	printOpenFileLimit()
	log.Printf("skip folders, %v", c.Skip.Folders)

	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to get current directory. Wtf?")
		os.Exit(1)
	}

	if _, err := os.Stat(config); err != nil {
		fmt.Fprintf(os.Stderr, "Please create %s.\n", config)
		os.Exit(1)
	}

	watcher, err := fsmonitor.NewWatcherWithSkipFolders(c.Skip.Folders)
	if err != nil {
		panic(err)
	}

	err = watcher.Watch(workingDir)
	commandTriggerDelays := make(map[string]time.Time)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for _ = range ch {
			fmt.Println(" Auf Wiederschaun!")
			os.Exit(0)
		}
	}()

	for _, w := range c.Watch {
		if w.Start == 0 {
			continue
		}
		log.Printf("Run %v ...\n", w.Command)
		cmd := exec.Command("sh", "-c", w.Command)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		go func() {
			// runMutex.Lock()
			// defer runMutex.Unlock()
			if err := cmd.Run(); err != nil {
				log.Printf("[error] [%v] %v", w.Pattern, err)
			}
		}()
		log.Printf("log: %s\n", w.Log)
		if w.Log != "" {
			logFile := w.Log
			go func() {
				t, err := tail.TailFile(logFile, tail.Config{Follow: true})
				if err != nil {
					panic(err)
				}
				log.Printf("tail -f %s\n", logFile)
				for line := range t.Lines {
					fmt.Println(line.Text)
				}
			}()
		}
	}

	for {
		select {
		case ev := <-watcher.Event:
			for _, w := range c.Watch {
				if ev.IsModify() {
					basename := path.Base(ev.Name)
					match, err := path.Match(w.Pattern, basename)
					if err != nil {
						log.Printf("[error] [%v] %v for pattern `%v`", basename, err, w.Pattern)
					}

					if match {
						if changed(ev.Name) {
							log.Printf("%s changed, match: %s", ev.Name, w.Pattern)
							last, ok := commandTriggerDelays[w.Pattern]
							if !ok || last.Add(time.Duration(w.Delay)*time.Millisecond).Before(time.Now()) {
								log.Printf("Run %v ...\n", w.Command)
								cmd := exec.Command("sh", "-c", w.Command)
								cmd.Stdin = os.Stdin
								cmd.Stdout = os.Stdout
								cmd.Stderr = os.Stderr
								commandTriggerDelays[w.Pattern] = time.Now()
								go func() {
									// runMutex.Lock()
									// defer runMutex.Unlock()
									if err := cmd.Run(); err != nil {
										log.Printf("[error] [%v] %v", basename, err)
									}
								}()
								log.Println("restart complete")
							}
						}
					}
				}
			}
		case err := <-watcher.Error:
			log.Printf("[error] %v", err)
		}
	}
}
