package main

import (
	"fmt"
	"os"

	"github.com/docopt/docopt-go"
	"github.com/mrxk/npv/internal/visualize"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	usage = `npv - Network Policy Visualizer

Usage:
	npv visualize [--namespace=<namespace>] [--ingress-only] [--egress-only]

Options:
	--namespace=<namespace>	Namespace
	`
)

type arguments struct {
	EgressOnly  bool
	IngressOnly bool
	Namespace   string
	Visualize   bool
}

func main() {
	opts := parseCli()
	args := bindOpts(opts)
	run(args)
}

func parseCli() docopt.Opts {
	opts, err := docopt.ParseDoc(usage)
	handleError(err)
	return opts
}

func bindOpts(opts docopt.Opts) arguments {
	args := arguments{}
	err := opts.Bind(&args)
	handleError(err)
	return args
}

func run(args arguments) {
	switch {
	case args.Visualize:
		handleError(runVisualize(args))
	}
}

func runVisualize(args arguments) error {
	clientset, err := getClientset(os.Getenv("KUBECONFIG"))
	if err != nil {
		return err
	}
	var category []string
	switch {
	case args.IngressOnly && args.EgressOnly:
		category = []string{"ingress", "egress"}
	case args.IngressOnly:
		category = []string{"ingress"}
	case args.EgressOnly:
		category = []string{"egress"}
	default:
		category = []string{"ingress", "egress"}
	}
	content, err := visualize.Visualize(args.Namespace, clientset, category)
	if err != nil {
		return err
	}
	fmt.Println(content)
	return nil
}

func handleError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func getClientset(kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}
