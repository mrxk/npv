package visualize_test

import (
	"io"
	"strings"
	"testing"

	"github.com/mrxk/npv/internal/visualize"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	one = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: one
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: pod2
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
      - podSelector:
          matchLabels:
            app: pod1
  egress:
    - to:
      - podSelector:
          matchLabels:
            app: pod2`
	oneExpected = `@startuml
left to right direction
frame Pods {
component "Name: one\lNamespace: default\lMatch Labels:\l    app: pod2\l" as defaultapppod2 {
    port "0-65535" as apppod1port
    portout " " as defaultapppod2portout
}
}
frame Ingress {
component "Pod:\l    Match Labels:\l        app: pod1\l" as apppod1 {
    portout " " as apppod1ingressportout
}
}
apppod1ingressportout --down[#green]--> apppod1port
frame Egress {
component "Pod:\l    Match Labels:\l        app: pod2\l" as apppod2 {
    port "0-65535" as apppod2egressport
}
}
defaultapppod2portout --down[#green]--> apppod2egressport
@enduml
`
	oneIngressOnlyExpected = `@startuml
left to right direction
frame Pods {
component "Name: one\lNamespace: default\lMatch Labels:\l    app: pod2\l" as defaultapppod2 {
    port "0-65535" as apppod1port
    portout " " as defaultapppod2portout
}
}
frame Ingress {
component "Pod:\l    Match Labels:\l        app: pod1\l" as apppod1 {
    portout " " as apppod1ingressportout
}
}
apppod1ingressportout --down[#green]--> apppod1port
@enduml
`
	oneEgressOnlyExpected = `@startuml
left to right direction
frame Pods {
component "Name: one\lNamespace: default\lMatch Labels:\l    app: pod2\l" as defaultapppod2 {
    port "0-65535" as apppod1port
    portout " " as defaultapppod2portout
}
}
frame Egress {
component "Pod:\l    Match Labels:\l        app: pod2\l" as apppod2 {
    port "0-65535" as apppod2egressport
}
}
defaultapppod2portout --down[#green]--> apppod2egressport
@enduml
`
	denyToPod = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: denyToPod
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: demo
  policyTypes:
    - Ingress
    - Egress`
	denyToPodExpected = `@startuml
left to right direction
frame Pods {
component "Name: denyToPod\lNamespace: default\lMatch Labels:\l    app: demo\l" as defaultappdemo {
    port "0-65535" as defaultappdemo_ALL_port
    portout " " as defaultappdemoportout
}
}
frame Ingress {
component "ALL" as _ALL_PEER_INGRESS_ {
    portout " " as _ALL_PEER_INGRESS_ingressportout
}
}
_ALL_PEER_INGRESS_ingressportout --down[#red]--> defaultappdemo_ALL_port
frame Egress {
component "ALL" as _ALL_ {
    port "0-65535" as _ALL_egressport
}
}
defaultappdemoportout --down[#red]--> _ALL_egressport
@enduml
`
	denyAll = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: denyAll
  namespace: default
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress`
	denyAllExpected = `@startuml
left to right direction
frame Pods {
component "Name: denyAll\lNamespace: default\lAll\l" as default_ALL_ {
    port "0-65535" as default_ALL__ALL_port
    portout " " as default_ALL_portout
}
}
frame Ingress {
component "ALL" as _ALL_PEER_INGRESS_ {
    portout " " as _ALL_PEER_INGRESS_ingressportout
}
}
_ALL_PEER_INGRESS_ingressportout --down[#red]--> default_ALL__ALL_port
frame Egress {
component "ALL" as _ALL_ {
    port "0-65535" as _ALL_egressport
}
}
default_ALL_portout --down[#red]--> _ALL_egressport
@enduml
`
	denyAllAndToPodExpected = `@startuml
left to right direction
frame Pods {
component "Name: denyAll\lNamespace: default\lAll\l" as default_ALL_ {
    port "0-65535" as default_ALL__ALL_port
    portout " " as default_ALL_portout
}
component "Name: denyToPod\lNamespace: default\lMatch Labels:\l    app: demo\l" as defaultappdemo {
    port "0-65535" as defaultappdemo_ALL_port
    portout " " as defaultappdemoportout
}
}
frame Ingress {
component "ALL" as _ALL_PEER_INGRESS_ {
    portout " " as _ALL_PEER_INGRESS_ingressportout
}
component "ALL" as _ALL_PEER_INGRESS_ {
    portout " " as _ALL_PEER_INGRESS_ingressportout
}
}
_ALL_PEER_INGRESS_ingressportout --down[#red]--> default_ALL__ALL_port
_ALL_PEER_INGRESS_ingressportout --down[#red]--> defaultappdemo_ALL_port
frame Egress {
component "ALL" as _ALL_ {
    port "0-65535" as _ALL_egressport
    port "0-65535" as _ALL_egressport
}
}
default_ALL_portout --down[#red]--> _ALL_egressport
defaultappdemoportout --down[#red]--> _ALL_egressport
@enduml
`
	allowToPod = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: network-policy
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: demo
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - {}
  egress:
    - {}
`
	allowToPodExpected = `@startuml
left to right direction
frame Pods {
component "Name: network-policy\lNamespace: default\lMatch Labels:\l    app: demo\l" as defaultappdemo {
    port "0-65535" as _ALL_PEER_port
    portout " " as defaultappdemoportout
}
}
frame Ingress {
component "ALL" as _ALL_PEER_INGRESS {
    portout " " as _ALL_PEER_INGRESSingressportout
}
}
_ALL_PEER_INGRESSingressportout --down[#green]--> _ALL_PEER_port
frame Egress {
component "ALL" as _ALL_PEER_ {
    port "0-65535" as _ALL_PEER_egressport
}
}
defaultappdemoportout --down[#green]--> _ALL_PEER_egressport
@enduml
`
	allInOne = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: all-in-one
  namespace: default
spec:
  podSelector:
    matchLabels:
      app: app1
  policyTypes:
  - Ingress
  - Egress
  egress:
  - ports:
    - port: 443
      protocol: TCP
    to:
    - ipBlock:
        cidr: 0.0.0.0/0
  - ports:
    - port: 1111
      protocol: TCP
    - port: 1112
      protocol: TCP
    - port: 1113
      protocol: TCP
    - port: 1114
      protocol: TCP
    - port: 1115
      protocol: TCP
    to:
    - ipBlock:
        cidr: 10.1.1.1/32
  - ports:
    - port: 443
      protocol: TCP
    to:
    - ipBlock:
        cidr: 10.1.1.2/32
    - ipBlock:
        cidr: 10.1.1.3/32
  - ports:
    - port: 443
      protocol: TCP
    to:
    - ipBlock:
        cidr: 10.1.1.4/32
  - ports:
    - port: 53
      protocol: UDP
    to:
    - namespaceSelector:
        matchLabels:
          namespace: other
      podSelector:
        matchLabels:
          app: app2
  - ports:
    - port: 1116
      protocol: TCP
    to:
    - podSelector:
        matchLabels:
          app: app3
  - ports:
    - port: 1117
      protocol: TCP
    to:
    - podSelector:
        matchLabels:
          app: app4
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - 10.1.1.5/32
        - 10.1.1.6/32
        - 10.1.1.7/32
        - 10.1.1.8/32
        - 10.1.1.9/32
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          namespace: other
      podSelector:
        matchLabels:
          app: app2
    ports:
    - port: 1118
      protocol: TCP
    - port: 1119
      protocol: TCP
    - port: 1121
      protocol: TCP
    - port: 1122
      protocol: TCP
    - port: 1123
      protocol: TCP
    - port: 1124
      protocol: TCP
    - port: 1125
      protocol: TCP
    - port: 1126
      protocol: TCP
    - port: 1127
      protocol: TCP
    - port: 1128
      protocol: TCP
  - from:
    - podSelector:
        matchLabels:
          app: app3
    ports:
    - port: 1129
      protocol: TCP
    - port: 1130
      protocol: TCP`

	allInOneExpected = `@startuml
left to right direction
frame Pods {
component "Name: all-in-one\lNamespace: default\lMatch Labels:\l    app: app1\l" as defaultappapp1 {
    port "1118 (TCP)" as appapp2namespaceotherTCP1118port
    port "1119 (TCP)" as appapp2namespaceotherTCP1119port
    port "1121 (TCP)" as appapp2namespaceotherTCP1121port
    port "1122 (TCP)" as appapp2namespaceotherTCP1122port
    port "1123 (TCP)" as appapp2namespaceotherTCP1123port
    port "1124 (TCP)" as appapp2namespaceotherTCP1124port
    port "1125 (TCP)" as appapp2namespaceotherTCP1125port
    port "1126 (TCP)" as appapp2namespaceotherTCP1126port
    port "1127 (TCP)" as appapp2namespaceotherTCP1127port
    port "1128 (TCP)" as appapp2namespaceotherTCP1128port
    port "1129 (TCP)" as appapp3TCP1129port
    port "1130 (TCP)" as appapp3TCP1130port
    portout " " as defaultappapp1portout
}
}
frame Ingress {
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceother {
    portout " " as appapp2namespaceotheringressportout
}
component "Pod:\l    Match Labels:\l        app: app3\l" as appapp3 {
    portout " " as appapp3ingressportout
}
component "Pod:\l    Match Labels:\l        app: app3\l" as appapp3 {
    portout " " as appapp3ingressportout
}
}
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1118port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1119port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1121port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1122port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1123port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1124port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1125port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1126port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1127port
appapp2namespaceotheringressportout --down[#green]--> appapp2namespaceotherTCP1128port
appapp3ingressportout --down[#green]--> appapp3TCP1129port
appapp3ingressportout --down[#green]--> appapp3TCP1130port
frame Egress {
component "IPBlock:\l    0.0.0.0/0 except 10.1.1.5/32, 10.1.1.6/32, 10.1.1.7/32, 10.1.1.8/32, 10.1.1.9/32\l" as 0.0.0.0_010.1.1.5_3210.1.1.6_3210.1.1.7_3210.1.1.8_3210.1.1.9_32 {
    port "0-65535" as 0.0.0.0_010.1.1.5_3210.1.1.6_3210.1.1.7_3210.1.1.8_3210.1.1.9_32egressport
}
component "IPBlock:\l    0.0.0.0/0\l" as 0.0.0.0_0TCP443 {
    port "443 (TCP)" as 0.0.0.0_0TCP443egressport
}
component "IPBlock:\l    10.1.1.1/32\l" as 10.1.1.1_32TCP1111 {
    port "1111 (TCP)" as 10.1.1.1_32TCP1111egressport
}
component "IPBlock:\l    10.1.1.1/32\l" as 10.1.1.1_32TCP1112 {
    port "1112 (TCP)" as 10.1.1.1_32TCP1112egressport
}
component "IPBlock:\l    10.1.1.1/32\l" as 10.1.1.1_32TCP1113 {
    port "1113 (TCP)" as 10.1.1.1_32TCP1113egressport
}
component "IPBlock:\l    10.1.1.1/32\l" as 10.1.1.1_32TCP1114 {
    port "1114 (TCP)" as 10.1.1.1_32TCP1114egressport
}
component "IPBlock:\l    10.1.1.1/32\l" as 10.1.1.1_32TCP1115 {
    port "1115 (TCP)" as 10.1.1.1_32TCP1115egressport
}
component "IPBlock:\l    10.1.1.2/32\l" as 10.1.1.2_32TCP443 {
    port "443 (TCP)" as 10.1.1.2_32TCP443egressport
}
component "IPBlock:\l    10.1.1.3/32\l" as 10.1.1.3_32TCP443 {
    port "443 (TCP)" as 10.1.1.3_32TCP443egressport
}
component "IPBlock:\l    10.1.1.4/32\l" as 10.1.1.4_32TCP443 {
    port "443 (TCP)" as 10.1.1.4_32TCP443egressport
}
component "Namespace:\l    Match Labels:\l        namespace: other\lPod:\l    Match Labels:\l        app: app2\l" as appapp2namespaceotherUDP53 {
    port "53 (UDP)" as appapp2namespaceotherUDP53egressport
}
component "Pod:\l    Match Labels:\l        app: app3\l" as appapp3TCP1116 {
    port "1116 (TCP)" as appapp3TCP1116egressport
}
component "Pod:\l    Match Labels:\l        app: app4\l" as appapp4TCP1117 {
    port "1117 (TCP)" as appapp4TCP1117egressport
}
}
defaultappapp1portout --down[#green]--> 0.0.0.0_010.1.1.5_3210.1.1.6_3210.1.1.7_3210.1.1.8_3210.1.1.9_32egressport
defaultappapp1portout --down[#green]--> 0.0.0.0_0TCP443egressport
defaultappapp1portout --down[#green]--> 10.1.1.1_32TCP1111egressport
defaultappapp1portout --down[#green]--> 10.1.1.1_32TCP1112egressport
defaultappapp1portout --down[#green]--> 10.1.1.1_32TCP1113egressport
defaultappapp1portout --down[#green]--> 10.1.1.1_32TCP1114egressport
defaultappapp1portout --down[#green]--> 10.1.1.1_32TCP1115egressport
defaultappapp1portout --down[#green]--> 10.1.1.2_32TCP443egressport
defaultappapp1portout --down[#green]--> 10.1.1.3_32TCP443egressport
defaultappapp1portout --down[#green]--> 10.1.1.4_32TCP443egressport
defaultappapp1portout --down[#green]--> appapp2namespaceotherUDP53egressport
defaultappapp1portout --down[#green]--> appapp3TCP1116egressport
defaultappapp1portout --down[#green]--> appapp4TCP1117egressport
@enduml
`
)

func TestVisaulize(t *testing.T) {
	tests := map[string]struct {
		policies      []string
		categories    []string
		namespace     string
		expected      string
		expectedError string
	}{
		"one": {
			policies: []string{
				one,
			},
			categories: []string{"ingress", "egress"},
			namespace:  "default",
			expected:   oneExpected,
		},
		"oneIngressOnly": {
			policies: []string{
				one,
			},
			categories: []string{"ingress"},
			namespace:  "default",
			expected:   oneIngressOnlyExpected,
		},
		"oneEgressOnly": {
			policies: []string{
				one,
			},
			categories: []string{"egress"},
			namespace:  "default",
			expected:   oneEgressOnlyExpected,
		},
		"denyToPod": {
			policies: []string{
				denyToPod,
			},
			categories: []string{"ingress", "egress"},
			namespace:  "default",
			expected:   denyToPodExpected,
		},
		"denyAll": {
			policies: []string{
				denyAll,
			},
			categories: []string{"ingress", "egress"},
			namespace:  "default",
			expected:   denyAllExpected,
		},
		"denyAllAndToPod": {
			policies: []string{
				denyToPod,
				denyAll,
			},
			categories: []string{"ingress", "egress"},
			namespace:  "default",
			expected:   denyAllAndToPodExpected,
		},
		"allowToPod": {
			policies: []string{
				allowToPod,
			},
			categories: []string{"ingress", "egress"},
			namespace:  "default",
			expected:   allowToPodExpected,
		},
		"allInOne": {
			policies: []string{
				allInOne,
			},
			categories: []string{"ingress", "egress"},
			namespace:  "default",
			expected:   allInOneExpected,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			clientset := createFakeClientset(t, tc.policies)
			actual, err := visualize.Visualize(tc.namespace, clientset, tc.categories)
			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual, actual)
			}
		})
	}
}

func createFakeClientset(t *testing.T, policies []string) *fake.Clientset {
	objects := []runtime.Object{}
	for _, policy := range policies {
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(policy), 32)
		var obj networkingv1.NetworkPolicy
		for {
			err := decoder.Decode(&obj)
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			objects = append(objects, &obj)
		}
	}
	return fake.NewClientset(objects...)
}
