package visualize

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mrxk/npv/internal/maputils"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

func VisualizeNamespaces(
	namespaces []string,
	clientset kubernetes.Interface,
	categories []string,
	linetype string,
) (string, error) {
	policies, err := getPoliciesFromNamespaces(namespaces, clientset)
	if err != nil {
		return "", err
	}
	podRules, err := convertToPodRules(policies)
	if err != nil {
		return "", err
	}
	return generatePlantUML(podRules, categories, linetype), nil
}

func VisualizeFiles(
	files,
	categories []string,
	linetype string,
) (string, error) {
	policies, err := getPoliciesFromFiles(files)
	if err != nil {
		return "", err
	}
	podRules, err := convertToPodRules(policies)
	if err != nil {
		return "", err
	}
	return generatePlantUML(podRules, categories, linetype), nil

}

type pod struct {
	id        string
	names     []string
	namespace string
	selector  metav1.LabelSelector
	ingress   []target
	egress    []target
}

func (p *pod) Label() string {
	b := strings.Builder{}
	b.WriteString("Name: " + strings.Join(p.names, ", ") + "\n")
	b.WriteString("Namespace: " + p.namespace + "\n")
	b.WriteString(selectorLabel("", p.selector))
	return strings.TrimSpace(b.String()) + "\n" // ensure one trailing newline
}

type target struct {
	id       string
	peerId   string
	peer     networkingv1.NetworkPolicyPeer
	port     networkingv1.NetworkPolicyPort
	blockAll bool
	allowAll bool
}

func (t *target) Label() string {
	if t.blockAll || t.allowAll {
		return "ALL"
	}
	b := strings.Builder{}
	b.WriteString(peerLabel(t.peer))
	// If the peer has a list of IPBlock exceptions then we need to add enough
	// newlines so PlantUml draws the box big enough to contain all the text.
	newlineCount := 1
	if t.peer.IPBlock != nil {
		newlineCount += len(t.peer.IPBlock.Except) / 2
	}
	return strings.TrimSpace(b.String()) + strings.Repeat("\n", newlineCount)
}

// compareTarget compares the string representation of targets.  It exists only
// to produce a stable order for benchmarking in tests.
func compareTarget(l, r target) int {
	return strings.Compare(l.id, r.id)
}

func convertToPodRules(policies []networkingv1.NetworkPolicy) (map[string]pod, error) {
	pods := map[string]pod{}
	for _, policy := range policies {
		key := podKey(policy.Namespace, policy.Spec.PodSelector)
		p, present := pods[key]
		if present {
			// This is another policy with the same selector
			p.names = append(p.names, policy.Name)
		}
		if !present {
			p = pod{
				id:        key,
				names:     []string{policy.Name},
				namespace: policy.Namespace,
				selector:  policy.Spec.PodSelector,
			}
		}
		if slices.Contains(policy.Spec.PolicyTypes, networkingv1.PolicyTypeIngress) {
			if len(policy.Spec.Ingress) == 0 {
				p.ingress = append(p.ingress, target{
					id:       p.id + "_ALL_",
					peerId:   "_ALL_PEER_INGRESS_",
					blockAll: true,
				})
			} else {
				for _, ingress := range policy.Spec.Ingress {
					if len(ingress.From) == 0 {
						t := target{
							id:       targetKey(networkingv1.NetworkPolicyPeer{}, networkingv1.NetworkPolicyPort{}),
							peerId:   "_ALL_PEER_INGRESS",
							peer:     networkingv1.NetworkPolicyPeer{},
							port:     networkingv1.NetworkPolicyPort{},
							allowAll: true,
						}
						p.ingress = append(p.ingress, t)
						continue
					}
					for _, peer := range ingress.From {
						if len(ingress.Ports) > 0 {
							for _, port := range ingress.Ports {
								t := target{
									id:     targetKey(peer, port),
									peerId: peerID(peer),
									peer:   peer,
									port:   port,
								}
								p.ingress = append(p.ingress, t)
							}
						} else {
							t := target{
								id:     targetKey(peer, networkingv1.NetworkPolicyPort{}),
								peerId: peerID(peer),
								peer:   peer,
								port:   networkingv1.NetworkPolicyPort{},
							}
							p.ingress = append(p.egress, t)
						}
					}
				}
			}
		}
		if slices.Contains(policy.Spec.PolicyTypes, networkingv1.PolicyTypeEgress) {
			if len(policy.Spec.Egress) == 0 {
				p.egress = append(p.egress, target{
					id:       p.id + "_ALL_",
					peerId:   "_ALL_PEER_EGRESS_",
					blockAll: true,
				})
			} else {
				for _, egress := range policy.Spec.Egress {
					if len(egress.To) == 0 {
						t := target{
							id:       targetKey(networkingv1.NetworkPolicyPeer{}, networkingv1.NetworkPolicyPort{}),
							peerId:   "_ALL_PEER_EGRESS_",
							peer:     networkingv1.NetworkPolicyPeer{},
							port:     networkingv1.NetworkPolicyPort{},
							allowAll: true,
						}
						p.egress = append(p.egress, t)
						continue
					}
					for _, peer := range egress.To {
						if len(egress.Ports) > 0 {
							for _, port := range egress.Ports {
								t := target{
									id:     targetKey(peer, port),
									peerId: peerID(peer),
									peer:   peer,
									port:   port,
								}
								p.egress = append(p.egress, t)
							}
						} else {
							t := target{
								id:     targetKey(peer, networkingv1.NetworkPolicyPort{}),
								peerId: peerID(peer),
								peer:   peer,
								port:   networkingv1.NetworkPolicyPort{},
							}
							p.egress = append(p.egress, t)
						}
					}
				}
			}
		}
		pods[key] = p
	}
	return sorted(pods), nil
}

func formatPort(port networkingv1.NetworkPolicyPort) string {
	switch {
	case port.Protocol != nil && port.Port != nil && port.EndPort != nil:
		return fmt.Sprintf("%s-%d (%s)", port.Port.String(), *port.EndPort, *port.Protocol)
	case port.Protocol != nil && port.Port != nil:
		return fmt.Sprintf("%s (%s)", port.Port.String(), *port.Protocol)
	case port.Port != nil && port.EndPort != nil:
		return fmt.Sprintf("%s-%d", port.Port.String(), *port.EndPort)
	case port.Port != nil:
		return port.Port.String()
	default:
		return "0-65535"
	}
}

func generatePodPlantUML(ids []string, pods map[string]pod, categories []string) string {
	b := strings.Builder{}
	b.WriteString("frame Pods {\n")
	for _, id := range ids {
		pod := pods[id]
		if (slices.Contains(categories, "ingress") && len(pod.ingress) > 0) ||
			(slices.Contains(categories, "egress") && len(pod.egress) > 0) {
			b.WriteString(fmt.Sprintf("component \"%s\" as %s {\n", strings.ReplaceAll(pod.Label(), "\n", "\\l"), id))
			if slices.Contains(categories, "ingress") {
				for _, t := range pod.ingress {
					if t.blockAll || t.allowAll {
						b.WriteString(fmt.Sprintf("    port \"0-65535\" as %s\n", t.id+"port"))
						continue
					}
					b.WriteString(fmt.Sprintf("    port \"%s\" as %s\n", formatPort(t.port), t.id+"port"))
				}
			}
			if slices.Contains(categories, "egress") {
				if len(pod.egress) > 0 {
					b.WriteString(fmt.Sprintf("    portout \" \" as %s\n", id+"portout"))
				}
			}
			b.WriteString("}\n")
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func generateIngressPlantUML(ids []string, pods map[string]pod) string {
	b := strings.Builder{}
	ingressNodes := map[string]struct{}{}
	b.WriteString("frame Ingress {\n")
	for _, id := range ids {
		pod := pods[id]
		for _, t := range pod.ingress {
			_, present := ingressNodes[t.peerId]
			if !present {
				b.WriteString(
					fmt.Sprintf("component \"%s\" as %s {\n    portout \" \" as %s\n}\n",
						strings.ReplaceAll(t.Label(), "\n", "\\l"),
						t.peerId+"_i",
						t.peerId+"ingressportout"),
				)
				ingressNodes[t.peerId] = struct{}{}
			}
		}
	}
	b.WriteString("}\n")
	// Create arrows to connect ingress to pods.
	for _, id := range ids {
		pod := pods[id]
		for _, t := range pod.ingress {
			if t.blockAll {
				b.WriteString(fmt.Sprintf("%s --down[#red]--> %s\n", t.peerId+"ingressportout", t.id+"port"))
			} else {
				b.WriteString(fmt.Sprintf("%s --down[#green]--> %s\n", t.peerId+"ingressportout", t.id+"port"))
			}
		}
	}
	return b.String()
}

func generateEgressPlantUML(ids []string, pods map[string]pod) string {
	b := strings.Builder{}
	egressComponents := map[string][]string{}
	for _, id := range ids {
		pod := pods[id]
		for _, t := range pod.egress {
			ports, present := egressComponents[t.peerId]
			if !present {
				ports = []string{fmt.Sprintf("component \"%s\" as %s {\n", strings.ReplaceAll(t.Label(), "\n", "\\l"), t.id+"_e")}
			}
			if t.blockAll || t.allowAll {
				ports = append(ports, fmt.Sprintf("    port \"0-65535\" as %s\n", t.id+"egressport"))
			} else {
				ports = append(ports, fmt.Sprintf("    port \"%s\" as %s\n", formatPort(t.port), t.id+"egressport"))
			}
			egressComponents[t.peerId] = ports
		}
	}
	b.WriteString("frame Egress {\n")
	for _, peerId := range maputils.SortedKeys(egressComponents) {
		b.WriteString(strings.Join(egressComponents[peerId], ""))
		b.WriteString("}\n")
	}
	b.WriteString("}\n")
	// Create arrows to connect pods to egress.
	for _, id := range ids {
		pod := pods[id]
		for _, t := range pod.egress {
			if t.blockAll {
				b.WriteString(fmt.Sprintf("%s --down[#red]--> %s\n", id+"portout", t.id+"egressport"))
			} else {
				b.WriteString(fmt.Sprintf("%s --down[#green]--> %s\n", id+"portout", t.id+"egressport"))
			}
		}
	}
	return b.String()
}

func generatePlantUML(
	pods map[string]pod,
	categories []string,
	linetype string,
) string {
	b := strings.Builder{}
	b.WriteString("@startuml\n")
	b.WriteString("left to right direction\n")
	if linetype != "" {
		b.WriteString(fmt.Sprintf("skinparam linetype %s\n", linetype))
	}
	ids := maputils.SortedKeys(pods)
	// Create the components that represent the pods.
	b.WriteString(generatePodPlantUML(ids, pods, categories))
	// Create components to represent all the ingres nodes
	if slices.Contains(categories, "ingress") {
		b.WriteString(generateIngressPlantUML(ids, pods))
	}
	// Create components to represent all the egress nodes
	if slices.Contains(categories, "egress") {
		b.WriteString(generateEgressPlantUML(ids, pods))
	}
	b.WriteString("@enduml\n")
	return b.String()
}

func getPoliciesFromNamespaces(namespaces []string, clientset kubernetes.Interface) ([]networkingv1.NetworkPolicy, error) {
	items := []networkingv1.NetworkPolicy{}
	if len(namespaces) == 0 {
		list, err := clientset.NetworkingV1().NetworkPolicies("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		return list.Items, nil

	}
	for _, namespace := range namespaces {
		list, err := clientset.NetworkingV1().NetworkPolicies(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		items = append(items, list.Items...)
	}
	return items, nil
}

func getPoliciesFromFiles(files []string) ([]networkingv1.NetworkPolicy, error) {
	items := []networkingv1.NetworkPolicy{}
	for _, file := range files {
		resolvedFiles, err := filepath.Glob(file)
		if err != nil {
			return nil, err
		}
		for _, resolvedFile := range resolvedFiles {
			contents, err := os.ReadFile(resolvedFile)
			if err != nil {
				return nil, err
			}
			decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(contents), 32)
			for {
				var obj networkingv1.NetworkPolicy
				err := decoder.Decode(&obj)
				if err == io.EOF {
					break
				}
				// Skip empty yaml documents
				if obj.Kind == "" {
					continue
				}
				items = append(items, obj)
			}
		}
	}
	return items, nil
}

func ipblockKey(ipblock networkingv1.IPBlock) string {
	parts := []string{ipblock.CIDR}
	parts = append(parts, ipblock.Except...)
	return normalizePlantUMLId(strings.Join(parts, ""))
}

func ipblockLable(i networkingv1.IPBlock) string {
	b := strings.Builder{}
	b.WriteString("    " + i.CIDR)
	if len(i.Except) > 0 {
		b.WriteString("\n        except:\n            " + strings.Join(i.Except, ",\n            "))
	}
	return b.String()
}

func labelSelectorID(selector metav1.LabelSelector) string {
	parts := []string{}
	if len(selector.MatchLabels) == 0 && len(selector.MatchExpressions) == 0 {
		return "_ALL_"
	}
	for _, k := range maputils.SortedKeys(selector.MatchLabels) {
		parts = append(parts, k, selector.MatchLabels[k])
	}
	for _, e := range selector.MatchExpressions {
		parts = append(parts, e.Key, string(e.Operator))
		parts = append(parts, e.Values...)
	}
	return normalizePlantUMLId(strings.Join(parts, ""))
}

func normalizePlantUMLId(v string) string {
	v = strings.ReplaceAll(v, "-", "_")
	v = strings.ReplaceAll(v, ":", "_")
	v = strings.ReplaceAll(v, "/", "_")
	return v
}

func peerID(peer networkingv1.NetworkPolicyPeer) string {
	parts := []string{}
	if peer.PodSelector != nil {
		parts = append(parts, labelSelectorID(*peer.PodSelector))
	}
	if peer.NamespaceSelector != nil {
		parts = append(parts, labelSelectorID(*peer.NamespaceSelector))
	}
	if peer.IPBlock != nil {
		parts = append(parts, ipblockKey(*peer.IPBlock))
	}
	key := normalizePlantUMLId(strings.Join(parts, ""))
	if key == "" {
		return "_ALL_PEER_"
	}
	return key
}

func peerLabel(p networkingv1.NetworkPolicyPeer) string {
	b := strings.Builder{}
	if p.NamespaceSelector != nil {
		b.WriteString("Namespace:\n" + selectorLabel("    ", *p.NamespaceSelector))
	}
	if p.PodSelector != nil {
		if p.NamespaceSelector != nil {
			b.WriteString("\n")
		}
		b.WriteString("Pod:\n" + selectorLabel("    ", *p.PodSelector))
	}
	if p.IPBlock != nil {
		if p.NamespaceSelector != nil || p.PodSelector != nil {
			b.WriteString("\n")
		}
		b.WriteString("IPBlock:\n" + ipblockLable(*p.IPBlock))
	}
	return b.String()
}

func podKey(namespace string, selector metav1.LabelSelector) string {
	parts := []string{namespace, labelSelectorID(selector)}
	return normalizePlantUMLId(strings.Join(parts, ""))
}

func selectorLabel(indent string, s metav1.LabelSelector) string {
	if len(s.MatchLabels) == 0 && len(s.MatchExpressions) == 0 {
		return indent + "All"
	}
	b := strings.Builder{}
	if len(s.MatchLabels) > 0 {
		b.WriteString(indent + "Match Labels:")
		for k, v := range s.MatchLabels {
			b.WriteString("\n" + indent + "    " + k + ": " + v)
		}
	}
	if len(s.MatchExpressions) > 0 {
		if len(s.MatchLabels) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(indent + "Match Expressions:")
		for _, e := range s.MatchExpressions {
			b.WriteString("\n" + indent + "    " + e.Key + " " + string(e.Operator) + " " + strings.Join(e.Values, ", "))
		}
	}
	return b.String()
}

// sorted does not sort the map of pods. It ensures that the fields of each pod
// are sorted so that benchmarks will be predictable.
func sorted(pods map[string]pod) map[string]pod {
	for _, pod := range pods {
		slices.Sort(pod.names)
		slices.SortFunc(pod.ingress, compareTarget)
		slices.SortFunc(pod.egress, compareTarget)
	}
	return pods
}

func targetKey(peer networkingv1.NetworkPolicyPeer, port networkingv1.NetworkPolicyPort) string {
	parts := []string{peerID(peer)}
	if port.Protocol != nil {
		parts = append(parts, string(*port.Protocol))
	}
	if port.Port != nil {
		parts = append(parts, port.Port.String())
	}
	if port.EndPort != nil {
		parts = append(parts, strconv.Itoa(int(*port.EndPort)))
	}
	return normalizePlantUMLId(strings.Join(parts, ""))
}
