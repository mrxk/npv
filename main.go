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
	npv visualize [(--namespace=<namespace>...|--file=<file>...)] [--out=<out>] [(--ingress-only|--egress-only)] [--linetype=<type>]

Options:
	--namespace=<namespace>	Namespace containing Network Policies to visualize
	--file=<file>	        Path to file containing Network Policies to visualize
	--out=<out>             Path to write visualiztion (- for stdout) (default: -)
	--ingress-only          Visualize only ingress rules
	--egress-only           Visualize only egress rules
	--linetype=<type>       Specify a line type (polyline or ortho)
	`
)

type arguments struct {
	EgressOnly  bool
	File        []string
	IngressOnly bool
	Namespace   []string
	Out         string
	Visualize   bool
	Linetype    string
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
	var category []string
	switch {
	case args.IngressOnly:
		category = []string{"ingress"}
	case args.EgressOnly:
		category = []string{"egress"}
	default:
		category = []string{"ingress", "egress"}
	}
	var content string
	var err error
	// If given files, then visualize files. Otherwise, assume visualization of
	// a cluster is desired.
	if len(args.File) > 0 {
		content, err = visualize.VisualizeFiles(args.File, category, args.Linetype)
	} else {
		var clientset *kubernetes.Clientset
		clientset, err = getClientset(os.Getenv("KUBECONFIG"))
		if err == nil {
			content, err = visualize.VisualizeNamespaces(args.Namespace, clientset, category, args.Linetype)
		}
	}
	if err != nil {
		return err
	}
	if len(args.Out) == 0 || args.Out == "-" {
		fmt.Println(content)
	} else {
		w, err := os.Create(args.Out)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, content)
	}
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
