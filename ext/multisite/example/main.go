// A minimal host: instances from a YAML file, routed by hostname, served by a toy runtime that answers with the
// instance it resolved. Real hosts plug in a provider instead (ext/pocker as isolates, ext/docker in containers).
//
//	go run ./example --instances instances.yaml
//	curl -H 'Host: blog.localhost' localhost:8090/
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"pocketone.dev/ext/multisite"
)

type echoRuntime struct{}

func (echoRuntime) Name() string                        { return "echo" }
func (echoRuntime) Serves(inst multisite.Instance) bool { return true }
func (echoRuntime) Ready() bool                         { return true }
func (echoRuntime) ServeInstance(w http.ResponseWriter, r *http.Request, inst multisite.Instance) {
	fmt.Fprintf(w, "instance %s (%s%s)\n", inst.ID, r.Host, r.URL.Path)
}

func main() {
	instances := flag.String("instances", "instances.yaml", "instances file: instances: [{id: blog, hosts: [blog.localhost]}]")
	listen := flag.String("listen", "127.0.0.1:8090", "listen address")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	h := &multisite.Host{Source: &multisite.YAMLSource{Path: *instances}, Runtime: echoRuntime{}, Listeners: multisite.Listeners{Listen: *listen}}
	if err := h.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
