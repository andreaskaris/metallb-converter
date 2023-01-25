package converter

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var validAddressPools0 = []metallbv1beta1.AddressPool{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ap-l2",
			Namespace: "metallb-system",
		},
		Spec: metallbv1beta1.AddressPoolSpec{
			Protocol:          ProtocolLayer2,
			Addresses:         []string{"192.168.100.100"},
			AutoAssign:        pointer.Bool(true),
			BGPAdvertisements: []metallbv1beta1.LegacyBgpAdvertisement{},
		},
		Status: metallbv1beta1.AddressPoolStatus{},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ap-bgp",
			Namespace: "metallb-system",
		},
		Spec: metallbv1beta1.AddressPoolSpec{
			Protocol:   ProtocolBGP,
			Addresses:  []string{"192.168.100.100"},
			AutoAssign: pointer.Bool(true),
			BGPAdvertisements: []metallbv1beta1.LegacyBgpAdvertisement{
				{
					AggregationLength:   pointer.Int32(32),
					AggregationLengthV6: pointer.Int32(64),
					LocalPref:           uint32(10),
					Communities:         []string{"65432:12345"},
				},
				{
					AggregationLength:   pointer.Int32(32),
					AggregationLengthV6: pointer.Int32(64),
					LocalPref:           uint32(11),
					Communities:         []string{"65433:12346"},
				},
			},
		},
		Status: metallbv1beta1.AddressPoolStatus{},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ap-bgp2",
			Namespace: "metallb-system",
		},
		Spec: metallbv1beta1.AddressPoolSpec{
			Protocol:          ProtocolBGP,
			Addresses:         []string{"192.168.100.100"},
			AutoAssign:        pointer.Bool(true),
			BGPAdvertisements: []metallbv1beta1.LegacyBgpAdvertisement{},
		},
		Status: metallbv1beta1.AddressPoolStatus{},
	},
}

var validAddressPools0YAML = `apiVersion: metallb.io/v1beta1
items:
- apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    creationTimestamp: null
    name: ap-bgp
    namespace: metallb-system
  spec:
    addresses:
    - 192.168.100.100
    autoAssign: true
  status: {}
- apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    creationTimestamp: null
    name: ap-bgp2
    namespace: metallb-system
  spec:
    addresses:
    - 192.168.100.100
    autoAssign: true
  status: {}
- apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    creationTimestamp: null
    name: ap-l2
    namespace: metallb-system
  spec:
    addresses:
    - 192.168.100.100
    autoAssign: true
  status: {}
kind: IPAddressPoolList
metadata: {}
---
apiVersion: metallb.io/v1beta1
items:
- apiVersion: metallb.io/v1beta1
  kind: L2Advertisement
  metadata:
    creationTimestamp: null
    name: ap-l2-l2-advertisement
    namespace: metallb-system
  spec:
    ipAddressPools:
    - ap-l2
  status: {}
kind: L2AdvertisementList
metadata: {}
---
apiVersion: metallb.io/v1beta1
items:
- apiVersion: metallb.io/v1beta1
  kind: BGPAdvertisement
  metadata:
    creationTimestamp: null
    name: ap-bgp-l2-advertisement-0
    namespace: metallb-system
  spec:
    aggregationLength: 32
    aggregationLengthV6: 64
    communities:
    - 65432:12345
    ipAddressPools:
    - ap-bgp
    localPref: 10
  status: {}
- apiVersion: metallb.io/v1beta1
  kind: BGPAdvertisement
  metadata:
    creationTimestamp: null
    name: ap-bgp-l2-advertisement-1
    namespace: metallb-system
  spec:
    aggregationLength: 32
    aggregationLengthV6: 64
    communities:
    - 65433:12346
    ipAddressPools:
    - ap-bgp
    localPref: 11
  status: {}
- apiVersion: metallb.io/v1beta1
  kind: BGPAdvertisement
  metadata:
    creationTimestamp: null
    name: ap-bgp2-l2-advertisement-0
    namespace: metallb-system
  spec:
    ipAddressPools:
    - ap-bgp2
  status: {}
kind: BGPAdvertisementList
metadata: {}`

var validAddressPools0Files = map[string]string{
	"IPAddressPoolList.yaml": `apiVersion: metallb.io/v1beta1
items:
- apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    creationTimestamp: null
    name: ap-bgp
    namespace: metallb-system
  spec:
    addresses:
    - 192.168.100.100
    autoAssign: true
  status: {}
- apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    creationTimestamp: null
    name: ap-bgp2
    namespace: metallb-system
  spec:
    addresses:
    - 192.168.100.100
    autoAssign: true
  status: {}
- apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    creationTimestamp: null
    name: ap-l2
    namespace: metallb-system
  spec:
    addresses:
    - 192.168.100.100
    autoAssign: true
  status: {}
kind: IPAddressPoolList
metadata: {}`,
	"L2AdvertisementList.yaml": `---
apiVersion: metallb.io/v1beta1
items:
- apiVersion: metallb.io/v1beta1
  kind: L2Advertisement
  metadata:
    creationTimestamp: null
    name: ap-l2-l2-advertisement
    namespace: metallb-system
  spec:
    ipAddressPools:
    - ap-l2
  status: {}
kind: L2AdvertisementList
metadata: {}`,
	"BGPAdvertisementList.yaml": `---
apiVersion: metallb.io/v1beta1
items:
- apiVersion: metallb.io/v1beta1
  kind: BGPAdvertisement
  metadata:
    creationTimestamp: null
    name: ap-bgp-l2-advertisement-0
    namespace: metallb-system
  spec:
    aggregationLength: 32
    aggregationLengthV6: 64
    communities:
    - 65432:12345
    ipAddressPools:
    - ap-bgp
    localPref: 10
  status: {}
- apiVersion: metallb.io/v1beta1
  kind: BGPAdvertisement
  metadata:
    creationTimestamp: null
    name: ap-bgp-l2-advertisement-1
    namespace: metallb-system
  spec:
    aggregationLength: 32
    aggregationLengthV6: 64
    communities:
    - 65433:12346
    ipAddressPools:
    - ap-bgp
    localPref: 11
  status: {}
- apiVersion: metallb.io/v1beta1
  kind: BGPAdvertisement
  metadata:
    creationTimestamp: null
    name: ap-bgp2-l2-advertisement-0
    namespace: metallb-system
  spec:
    ipAddressPools:
    - ap-bgp2
  status: {}
kind: BGPAdvertisementList
metadata: {}`,
}

var validAddressPools1 = []metallbv1beta1.AddressPool{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ap-bgp",
			Namespace: "metallb-system",
		},
		Spec: metallbv1beta1.AddressPoolSpec{
			Protocol:          ProtocolBGP,
			Addresses:         []string{"192.168.100.100"},
			AutoAssign:        pointer.Bool(true),
			BGPAdvertisements: []metallbv1beta1.LegacyBgpAdvertisement{}},
		Status: metallbv1beta1.AddressPoolStatus{},
	},
}

var validAddressPools1YAML = `apiVersion: metallb.io/v1beta1
items:
- apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    creationTimestamp: null
    name: ap-bgp
    namespace: metallb-system
  spec:
    addresses:
    - 192.168.100.100
    autoAssign: true
  status: {}
kind: IPAddressPoolList
metadata: {}
---
apiVersion: metallb.io/v1beta1
items:
- apiVersion: metallb.io/v1beta1
  kind: BGPAdvertisement
  metadata:
    creationTimestamp: null
    name: ap-bgp-l2-advertisement-0
    namespace: metallb-system
  spec:
    ipAddressPools:
    - ap-bgp
  status: {}
kind: BGPAdvertisementList
metadata: {}`

// This is expected to match validAddressPool0 but in its file representation.
var validAddressPoolFiles = map[string]string{
	"bgp-addresspools.yaml": `apiVersion: metallb.io/v1beta1
kind: AddressPool
metadata:
  name: bgp4
  namespace: metallb-system
spec:
  addresses:
  - 192.168.0.100-192.168.0.103
  autoAssign: true
  protocol: bgp
  bgpAdvertisements:
    - communities: 
       - 65535:65282`,
	"bgp-addresspools2.yaml": `apiVersion: metallb.io/v1beta1
kind: AddressPool
metadata:
  name: bgp6
  namespace: metallb-system
spec:
  addresses:
  - 2000::100-2000::103
  autoAssign: true
  protocol: bgp
  bgpAdvertisements:
    - communities: 
       - 65535:65282`,
	"l2-addresspools.yaml": `apiVersion: metallb.io/v1beta1
kind: AddressPool
metadata:
  name: l24
  namespace: metallb-system
spec:
  addresses:
  - 192.168.0.200-192.168.0.203
  autoAssign: true
  protocol: layer2
---
apiVersion: metallb.io/v1beta1
kind: AddressPool
metadata:
  name: l26
  namespace: metallb-system
spec:
  addresses:
  - 2000::200-2000::203
  autoAssign: true
  protocol: layer2`,
}

func TestReadLegacyObjectsFromAPI(t *testing.T) {
	var scheme = runtime.NewScheme()
	err := metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		log.Fatal(err)
	}

	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	for _, ap := range validAddressPools0 {
		err := c.Create(context.TODO(), &ap)
		if err != nil {
			t.Fatal(err)
		}
	}

	legacyObjects, err := ReadLegacyObjectsFromAPI(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyObjects.AddressPoolList.Items) != len(validAddressPools0) {
		t.Fatalf("mismatch between created and retrieved address pools, got: %v, expected: %v",
			legacyObjects.AddressPoolList.Items, validAddressPools0)
	}
}

func TestReadLegacyObjectsFromDirectory(t *testing.T) {
	var scheme = runtime.NewScheme()
	err := metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		log.Fatal(err)
	}

	tcs := map[string]struct {
		dir                  string
		addressPoolFiles     map[string]string
		expectedOutputLength int
		expectedErrorString  string
	}{
		"valid test case": {
			dir:                  "tmpDir",
			addressPoolFiles:     validAddressPoolFiles,
			expectedOutputLength: 4,
			expectedErrorString:  "",
		},
		"invalid test case": {
			dir:                  "/tmp/converter_test_zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			addressPoolFiles:     validAddressPoolFiles,
			expectedOutputLength: 0,
			expectedErrorString:  "no such file or directory",
		},
	}
	for desc, tc := range tcs {
		tmpDir := tc.dir
		if tc.dir == "tmpDir" {
			tmpDir = t.TempDir()
			for fileName, fileContent := range tc.addressPoolFiles {
				err := os.WriteFile(path.Join(tmpDir, fileName), []byte(fileContent), 0644)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		legacyObjects, err := ReadLegacyObjectsFromDirectory(scheme, tmpDir)
		if tc.expectedErrorString != "" && err == nil ||
			err != nil && tc.expectedErrorString == "" ||
			err != nil && !strings.Contains(err.Error(), tc.expectedErrorString) {
			t.Fatalf("TestReadLebacyObjects(%s): Generated error does not match expected error. Expected %q but got %q",
				desc, tc.expectedErrorString, err)
		}
		if err == nil && len(legacyObjects.AddressPoolList.Items) != tc.expectedOutputLength {
			t.Fatalf("TestReadLebacyObjects(%s): mismatch between created and retrieved address pools, got: %v, expected: %v",
				desc, legacyObjects.AddressPoolList.Items, tc.addressPoolFiles)
		}
	}
}

func TestConvert(t *testing.T) {
	// Build client.
	var scheme = runtime.NewScheme()
	err := metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		log.Fatal(err)
	}

	tcs := map[string]struct {
		addressPoolList     []metallbv1beta1.AddressPool
		expectedOutput      string
		expectedTargetFiles map[string]string
		expectedErrorString string
	}{
		"valid test case 0": {
			addressPoolList:     validAddressPools0,
			expectedOutput:      validAddressPools0YAML,
			expectedErrorString: "",
		},
		"valid test case 1": {
			addressPoolList:     validAddressPools1,
			expectedOutput:      validAddressPools1YAML,
			expectedErrorString: "",
		},
		"valid test case 2": {
			addressPoolList:     validAddressPools0,
			expectedTargetFiles: validAddressPools0Files,
			expectedErrorString: "",
		},
	}
	for desc, tc := range tcs {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		for _, ap := range tc.addressPoolList {
			err := c.Create(context.TODO(), &ap)
			if err != nil {
				t.Fatal(err)
			}
		}
		// Read.
		legacyObjects, err := ReadLegacyObjectsFromAPI(c)
		if err != nil {
			t.Fatal(err)
		}
		// Convert.
		currentObjects, err := Convert(legacyObjects)
		if err != nil {
			log.Fatal(err)
		}
		// Print step.
		stdout = bytes.NewBuffer([]byte{})
		targetDir := ""
		if len(tc.expectedTargetFiles) > 0 {
			targetDir = t.TempDir()
		}
		err = PrintCurrentObjects(currentObjects, targetDir, false)
		if err != nil {
			t.Fatal(err)
		}
		if tc.expectedErrorString != "" && err == nil ||
			err != nil && tc.expectedErrorString == "" ||
			err != nil && !strings.Contains(err.Error(), tc.expectedErrorString) {
			t.Fatalf("TestConvert(%s): Generated error does not match expected error. Expected %q but got %q",
				desc, tc.expectedErrorString, err)
		}
		if err == nil && fmt.Sprint(stdout) != tc.expectedOutput {
			t.Fatalf("TestConvert(%s): Generated output does not match expected output.\nGenerated output:\n===\n```%s```\n\nExpected output:\n===\n```%s```",
				desc, stdout, tc.expectedOutput)
		}
		if len(tc.expectedTargetFiles) > 0 {
			for expectedFileName, expectedFileContent := range tc.expectedTargetFiles {
				generatedContent, err := os.ReadFile(path.Join(targetDir, expectedFileName))
				if err != nil {
					t.Fatalf("TestConvert(%s): Could not read expected file %s, err: %q", desc, expectedFileName, err)
				}
				if expectedFileContent != string(generatedContent) {
					t.Fatalf("TestConvert(%s): File content mismatch for file %s.\nGot\n'%s'\nExpected\n'%s'",
						desc, expectedFileName, generatedContent, expectedFileContent)
				}
			}
		}
	}
}

func TestPrintObj(t *testing.T) {
	tcs := map[string]struct {
		obj    runtime.Object
		errStr string
	}{
		"test invalid object": {
			obj: &metallbv1beta1.AddressPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ap-l2",
					Namespace: "metallb-system",
				},
				Spec: metallbv1beta1.AddressPoolSpec{
					Protocol:          ProtocolLayer2,
					Addresses:         []string{"192.168.100.100"},
					AutoAssign:        pointer.Bool(true),
					BGPAdvertisements: []metallbv1beta1.LegacyBgpAdvertisement{},
				},
				Status: metallbv1beta1.AddressPoolStatus{},
			},
			errStr: "missing apiVersion or kind; try GetObjectKind().SetGroupVersionKind() if you know the type",
		},
	}
	printer := &printers.YAMLPrinter{}
	for desc, tc := range tcs {
		output, err := printObj(tc.obj, printer)
		if tc.errStr == "" && err != nil ||
			err != nil && tc.errStr == "" ||
			err != nil && !strings.Contains(err.Error(), tc.errStr) {
			t.Fatalf("TestPrintObj(%s): failed due to returned error %q does not match expected error message %s",
				desc, err, tc.errStr)
		}
		if tc.errStr == "" && output == "" {
			t.Fatalf("TestPrintObj(%s): failed due to returned string being the empty string", desc)
		}
	}
}
