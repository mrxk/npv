package visualize_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/mrxk/npv/internal/visualize"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"
)

var tests = map[string]struct {
	policies      []string
	categories    []string
	namespace     []string
	expected      string
	expectedError string
}{
	"one": {
		policies: []string{
			"testdata/allowToPod.input",
		},
		categories: []string{"ingress", "egress"},
		namespace:  []string{"default"},
		expected:   "testdata/allowToPod.expected",
	},
	"oneIngressOnly": {
		policies: []string{
			"testdata/allowToPod.input",
		},
		categories: []string{"ingress"},
		namespace:  []string{"default"},
		expected:   "testdata/allowToPod.ingress.expected",
	},
	"oneEgressOnly": {
		policies: []string{
			"testdata/allowToPod.input",
		},
		categories: []string{"egress"},
		namespace:  []string{"default"},
		expected:   "testdata/allowToPod.egress.expected",
	},
	"denyToPod": {
		policies: []string{
			"testdata/denyToPod.input",
		},
		categories: []string{"ingress", "egress"},
		namespace:  []string{"default"},
		expected:   "testdata/denyToPod.expected",
	},
	"denyAll": {
		policies: []string{
			"testdata/denyAll.input",
		},
		categories: []string{"ingress", "egress"},
		namespace:  []string{"default"},
		expected:   "testdata/denyAll.expected",
	},
	"denyAllAndToPod": {
		policies: []string{
			"testdata/denyAll.input",
			"testdata/denyToPod.input",
		},
		categories: []string{"ingress", "egress"},
		namespace:  []string{"default"},
		expected:   "testdata/denyAllAndToPod.expected",
	},
	"allowAll": {
		policies: []string{
			"testdata/allowAll.input",
		},
		categories: []string{"ingress", "egress"},
		namespace:  []string{"default"},
		expected:   "testdata/allowAll.expected",
	},
	"allInOne": {
		policies: []string{
			"testdata/allInOne.input",
		},
		categories: []string{"ingress", "egress"},
		namespace:  []string{"default"},
		expected:   "testdata/allInOne.expected",
	},
	"multiple": {
		policies: []string{
			"testdata/multiple.input",
		},
		categories: []string{"ingress", "egress"},
		namespace:  []string{"default"},
		expected:   "testdata/multiple.expected",
	},
	"multipleNamespaces": {
		policies: []string{
			"testdata/multipleNamespaces.input",
		},
		categories: []string{"ingress", "egress"},
		namespace:  []string{"default", "one", "two"},
		expected:   "testdata/multipleNamespaces.expected",
	},
}

func TestVisaulizeNamepsaces(t *testing.T) {
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			clientset := createFakeClientset(t, tc.policies)
			actual, err := visualize.VisualizeNamespaces(tc.namespace, clientset, tc.categories)
			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
				expected, err := os.ReadFile(tc.expected)
				require.NoError(t, err)
				require.Equal(t, string(expected), actual, actual)
			}
		})
	}
}

func TestVisaulizeFiles(t *testing.T) {
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual, err := visualize.VisualizeFiles(tc.policies, tc.categories)
			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
				expected, err := os.ReadFile(tc.expected)
				require.NoError(t, err)
				require.Equal(t, string(expected), actual, actual)
			}
		})
	}
}

func createFakeClientset(t *testing.T, policies []string) *fake.Clientset {
	objects := []runtime.Object{}
	for _, policy := range policies {
		contents, err := os.ReadFile(policy)
		require.NoError(t, err)
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(contents), 32)
		for {
			var obj networkingv1.NetworkPolicy
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
