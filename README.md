# NPV - NetworkPolicyViewer

This project fetches NetworkPolicy resources from the Kubernetes cluster
identified by the current `KUBECONFIG` environment variable and prints a
[PlantUML](http://www.plantuml.com) component diagram to stdout.

## Install

`go install github.com/mrxk/npv@latest`

## Usage

`npv visualize [--namespace=namespace] [--ingress-only] [--egress-only]`

If not given a namespace, all NetworkPolicy resources in the cluster will be
fetched. The output can be filtered to only ingress or egress rules with the
cooresponding options.

The output can be saved and processed with PlantUML or piped directly to it.

`npv visualize --namespace default | java -jar plantuml-1.2024.8.jar -pipe > default.png`

PlantUML can be downloaded from
[https://plantuml.com/download](https://plantuml.com/download).

## Build

1. Clone the project
1. Build with `go build`

## Examples


![allowToPod](allowToPod.png)

![allowToPod.ingress](allowToPod.ingress.png)

![allowToPod.egress](allowToPod.egress.png)

![allowAll](allowAll.png)

![denyToPod](denyToPod.png)

![denyAll](denyAll.png)

![denyAllAndToPod](denyAllAndToPod.png)

![multiple](multiple.png)

![allInOne](allInOne.png)
