// Copyright 2018 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/rjeczalik/notify"
)

type Event struct {
	Event, Path string
}

func main() {
	flagCalmdown := flag.Duration("calmdown", 2*time.Second, "calmdown period to wait after events and before calling the command")
	flagCmdArgs := flag.String("args", "echo {{.Path}}", "command to execute (with sh -c)")
	flag.Parse()
	tmpl := template.Must(template.New("").Parse(*flagCmdArgs))

	ch := make(chan notify.EventInfo, 1)
	for _, path := range flag.Args() {
		log.Printf("watching %q", path)
		if err := notify.Watch(path, ch, notify.InMovedTo|notify.InCloseWrite); err != nil {
			log.Fatal(err)
		}
	}
	wait := time.NewTimer(*flagCalmdown)
	if !wait.Stop() {
		<-wait.C
	}
	evtCh := make(chan string)
	go func() {
		var args string
		for {
			var ok bool
			select {
			case args, ok = <-evtCh:
				if !ok {
					return
				}
			case <-wait.C:
				if args == "" {
					continue
				}
				cmd := exec.Command("sh", "-c", args)
				cmd.Stdin = nil
				cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
				log.Printf("executing %q", cmd.Args)
				if err := cmd.Run(); err != nil {
					log.Println(cmd.Args, err)
				}
			}
		}
	}()

	defer close(evtCh)
	var buf bytes.Buffer
	for ei := range ch {
		evt := Event{Event: ei.Event().String(), Path: ei.Path()}
		log.Println("received", evt)
		buf.Reset()
		if err := tmpl.Execute(&buf, evt); err != nil {
			log.Fatal(err)
		}

		if !wait.Stop() {
			select {
			case <-wait.C:
			default:
			}
		}
		wait.Reset(*flagCalmdown)

		select {
		case evtCh <- buf.String():
		default:
			log.Println("skip", evt)
		}
	}
}
