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
	"sigs.k8s.io/controller-runtime/pkg/client"
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

var validAddressPools0BackupFiles = map[string]string{
	"AddressPool.yaml": `apiVersion: metallb.io/v1beta1
kind: AddressPool
metadata:
  creationTimestamp: null
  name: ap-bgp
  namespace: metallb-system
spec:
  addresses:
  - 192.168.100.100
  autoAssign: true
  bgpAdvertisements:
  - aggregationLength: 32
    aggregationLengthV6: 64
    communities:
    - 65432:12345
    localPref: 10
  - aggregationLength: 32
    aggregationLengthV6: 64
    communities:
    - 65433:12346
    localPref: 11
  protocol: bgp
status: {}
---
apiVersion: metallb.io/v1beta1
kind: AddressPool
metadata:
  creationTimestamp: null
  name: ap-bgp2
  namespace: metallb-system
spec:
  addresses:
  - 192.168.100.100
  autoAssign: true
  protocol: bgp
status: {}
---
apiVersion: metallb.io/v1beta1
kind: AddressPool
metadata:
  creationTimestamp: null
  name: ap-l2
  namespace: metallb-system
spec:
  addresses:
  - 192.168.100.100
  autoAssign: true
  protocol: layer2
status: {}
`,
}

var validAddressPools0YAML = `apiVersion: metallb.io/v1beta1
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
---
apiVersion: metallb.io/v1beta1
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
---
apiVersion: metallb.io/v1beta1
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
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  creationTimestamp: null
  name: ap-l2-l2-advertisement
  namespace: metallb-system
spec:
  ipAddressPools:
  - ap-l2
status: {}
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: ap-bgp-bgp-advertisement-0
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
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: ap-bgp-bgp-advertisement-1
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
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: ap-bgp2-bgp-advertisement-0
  namespace: metallb-system
spec:
  ipAddressPools:
  - ap-bgp2
status: {}
`

var validAddressPools0JSON = `{
    "kind": "IPAddressPool",
    "apiVersion": "metallb.io/v1beta1",
    "metadata": {
        "name": "ap-bgp",
        "namespace": "metallb-system",
        "creationTimestamp": null
    },
    "spec": {
        "addresses": [
            "192.168.100.100"
        ],
        "autoAssign": true
    },
    "status": {}
}
{
    "kind": "IPAddressPool",
    "apiVersion": "metallb.io/v1beta1",
    "metadata": {
        "name": "ap-bgp2",
        "namespace": "metallb-system",
        "creationTimestamp": null
    },
    "spec": {
        "addresses": [
            "192.168.100.100"
        ],
        "autoAssign": true
    },
    "status": {}
}
{
    "kind": "IPAddressPool",
    "apiVersion": "metallb.io/v1beta1",
    "metadata": {
        "name": "ap-l2",
        "namespace": "metallb-system",
        "creationTimestamp": null
    },
    "spec": {
        "addresses": [
            "192.168.100.100"
        ],
        "autoAssign": true
    },
    "status": {}
}
{
    "kind": "L2Advertisement",
    "apiVersion": "metallb.io/v1beta1",
    "metadata": {
        "name": "ap-l2-l2-advertisement",
        "namespace": "metallb-system",
        "creationTimestamp": null
    },
    "spec": {
        "ipAddressPools": [
            "ap-l2"
        ]
    },
    "status": {}
}
{
    "kind": "BGPAdvertisement",
    "apiVersion": "metallb.io/v1beta1",
    "metadata": {
        "name": "ap-bgp-bgp-advertisement-0",
        "namespace": "metallb-system",
        "creationTimestamp": null
    },
    "spec": {
        "aggregationLength": 32,
        "aggregationLengthV6": 64,
        "localPref": 10,
        "communities": [
            "65432:12345"
        ],
        "ipAddressPools": [
            "ap-bgp"
        ]
    },
    "status": {}
}
{
    "kind": "BGPAdvertisement",
    "apiVersion": "metallb.io/v1beta1",
    "metadata": {
        "name": "ap-bgp-bgp-advertisement-1",
        "namespace": "metallb-system",
        "creationTimestamp": null
    },
    "spec": {
        "aggregationLength": 32,
        "aggregationLengthV6": 64,
        "localPref": 11,
        "communities": [
            "65433:12346"
        ],
        "ipAddressPools": [
            "ap-bgp"
        ]
    },
    "status": {}
}
{
    "kind": "BGPAdvertisement",
    "apiVersion": "metallb.io/v1beta1",
    "metadata": {
        "name": "ap-bgp2-bgp-advertisement-0",
        "namespace": "metallb-system",
        "creationTimestamp": null
    },
    "spec": {
        "ipAddressPools": [
            "ap-bgp2"
        ]
    },
    "status": {}
}
`

var validAddressPools0Files = map[string]string{
	"IPAddressPool.yaml": `apiVersion: metallb.io/v1beta1
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
---
apiVersion: metallb.io/v1beta1
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
---
apiVersion: metallb.io/v1beta1
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
`,
	"L2Advertisement.yaml": `apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  creationTimestamp: null
  name: ap-l2-l2-advertisement
  namespace: metallb-system
spec:
  ipAddressPools:
  - ap-l2
status: {}
`,
	"BGPAdvertisement.yaml": `apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: ap-bgp-bgp-advertisement-0
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
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: ap-bgp-bgp-advertisement-1
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
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: ap-bgp2-bgp-advertisement-0
  namespace: metallb-system
spec:
  ipAddressPools:
  - ap-bgp2
status: {}
`,
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
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: ap-bgp-bgp-advertisement-0
  namespace: metallb-system
spec:
  ipAddressPools:
  - ap-bgp
status: {}
`

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
  protocol: layer2
`,
}

var validAddressPoolFilesYAML = `apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  creationTimestamp: null
  name: bgp4
  namespace: metallb-system
spec:
  addresses:
  - 192.168.0.100-192.168.0.103
  autoAssign: true
status: {}
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  creationTimestamp: null
  name: bgp6
  namespace: metallb-system
spec:
  addresses:
  - 2000::100-2000::103
  autoAssign: true
status: {}
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  creationTimestamp: null
  name: l24
  namespace: metallb-system
spec:
  addresses:
  - 192.168.0.200-192.168.0.203
  autoAssign: true
status: {}
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  creationTimestamp: null
  name: l26
  namespace: metallb-system
spec:
  addresses:
  - 2000::200-2000::203
  autoAssign: true
status: {}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  creationTimestamp: null
  name: l24-l2-advertisement
  namespace: metallb-system
spec:
  ipAddressPools:
  - l24
status: {}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  creationTimestamp: null
  name: l26-l2-advertisement
  namespace: metallb-system
spec:
  ipAddressPools:
  - l26
status: {}
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: bgp4-bgp-advertisement-0
  namespace: metallb-system
spec:
  communities:
  - 65535:65282
  ipAddressPools:
  - bgp4
status: {}
---
apiVersion: metallb.io/v1beta1
kind: BGPAdvertisement
metadata:
  creationTimestamp: null
  name: bgp6-bgp-advertisement-0
  namespace: metallb-system
spec:
  communities:
  - 65535:65282
  ipAddressPools:
  - bgp6
status: {}
`

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

	legacyObjects, err := ReadLegacyObjectsFromAPI(c, 0)
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

func TestOfflineMigration(t *testing.T) {
	var scheme = runtime.NewScheme()
	err := metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		log.Fatal(err)
	}

	tcs := map[string]struct {
		addressPoolList     []metallbv1beta1.AddressPool
		inputFiles          map[string]string
		expectedOutput      string
		expectedTargetFiles map[string]string
		expectedErrorString string
		json                bool
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
		"valid test case 3": {
			addressPoolList:     validAddressPools0,
			expectedOutput:      validAddressPools0JSON,
			expectedErrorString: "",
			json:                true,
		},
		"valid test case 4": {
			inputFiles:          validAddressPoolFiles,
			expectedOutput:      validAddressPoolFilesYAML,
			expectedErrorString: "",
			json:                false,
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

		// Create a fake stdout.
		stdout = bytes.NewBuffer([]byte{})
		// Create the targetDir if needed.
		targetDir := ""
		if len(tc.expectedTargetFiles) > 0 {
			targetDir = t.TempDir()
		}
		// Create the sourceDir if needed.
		sourceDir := ""
		if len(tc.inputFiles) > 0 {
			sourceDir = t.TempDir()
			for fileName, fileContent := range tc.inputFiles {
				err := os.WriteFile(path.Join(sourceDir, fileName), []byte(fileContent), 0644)
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		err := OfflineMigration(c, scheme, sourceDir, targetDir, tc.json)
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

// TODO: The transformer function at the moment does not do anything. Address this at some point and test failures.
func TestOnlineMigration(t *testing.T) {
	json := false
	var scheme = runtime.NewScheme()
	err := metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		log.Fatal(err)
	}

	tcs := map[string]struct {
		inputAddressPoolList        []metallbv1beta1.AddressPool
		outputAddressPoolCount      int
		outputIPAddressPoolCount    int
		outputBGPAdvertisementCount int
		outputL2AdvertisementCount  int
		errorStr                    string
		transformerFunc             func(client.Client)
		backupDir                   string
		expectedBackupFiles         map[string]string
	}{
		"test case 0": {
			inputAddressPoolList:        validAddressPools0,
			outputAddressPoolCount:      0,
			outputIPAddressPoolCount:    3,
			outputBGPAdvertisementCount: 3,
			outputL2AdvertisementCount:  1,
			transformerFunc:             nil,
		},
		"test case 1": {
			inputAddressPoolList:        validAddressPools0,
			outputAddressPoolCount:      0,
			outputIPAddressPoolCount:    3,
			outputBGPAdvertisementCount: 3,
			outputL2AdvertisementCount:  1,
			transformerFunc: func(c client.Client) {
				var apl metallbv1beta1.AddressPoolList
				err := c.List(context.TODO(), &apl)
				if err != nil {
					t.Fatalf("TestOnlineMigration(test case 1): got error in transformer function on listing, err: %q",
						err)
				}
				if len(apl.Items) != 3 {
					t.Fatalf("TestOnlineMigration(test case 1): got error in  transformer function, expected to see 3 "+
						"AddressPools but got %d", len(apl.Items))
				}
				err = c.Delete(context.TODO(), &apl.Items[0])
				if err != nil {
					t.Fatalf("TestOnlineMigration(test case 1): got error in transformer function on deletion, err: %q",
						err)
				}
			},
		},
		"test case 2": {
			inputAddressPoolList:        validAddressPools0,
			outputAddressPoolCount:      0,
			outputIPAddressPoolCount:    3,
			outputBGPAdvertisementCount: 3,
			outputL2AdvertisementCount:  1,
			transformerFunc:             nil,
			backupDir:                   "tmpDir",
			expectedBackupFiles:         validAddressPools0BackupFiles,
		},
	}
	for desc, tc := range tcs {
		// Determine backup dir (and create temp dir if necessary).
		backupDir := tc.backupDir
		if tc.backupDir == "tmpDir" {
			backupDir = t.TempDir()
		}
		// Build scheme.
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		// Create objects in fake API.
		for _, ap := range tc.inputAddressPoolList {
			err := c.Create(context.TODO(), &ap)
			if err != nil {
				t.Fatal(err)
			}
		}
		// Migration step.
		err = OnlineMigration(c, scheme, backupDir, json)
		if err != nil {
			if tc.errorStr == "" || !strings.Contains(err.Error(), tc.errorStr) {
				log.Fatalf("TestOnlineMigration(%s): expected error does not match. Expected: %q but got %q", desc,
					tc.errorStr, err)
			}
			continue
		} else if tc.errorStr != "" {
			log.Fatalf("TestOnlineMigration(%s): expected error but got none instead", desc)
		}
		// Make sure that backup files were correctly written.
		if backupDir != "" {
			for expectedFileName, expectedFileContent := range tc.expectedBackupFiles {
				generatedContent, err := os.ReadFile(path.Join(backupDir, expectedFileName))
				if err != nil {
					t.Fatalf("TestConvert(%s): Could not read expected file %s, err: %q", desc, expectedFileName, err)
				}
				if expectedFileContent != string(generatedContent) {
					t.Fatalf("TestConvert(%s): File content mismatch for file %s.\nGot\n'%s'\nExpected\n'%s'",
						desc, expectedFileName, generatedContent, expectedFileContent)
				}
			}
		}
		// Read results from fake API.
		var outputAddressPoolList metallbv1beta1.AddressPoolList
		var outputIPAddressPoolList metallbv1beta1.IPAddressPoolList
		var outputBGPAdvertisementList metallbv1beta1.BGPAdvertisementList
		var outputL2AdvertisementList metallbv1beta1.L2AdvertisementList
		err = c.List(context.TODO(), &outputAddressPoolList)
		if err != nil {
			log.Fatal(err)
		}
		err = c.List(context.TODO(), &outputIPAddressPoolList)
		if err != nil {
			log.Fatal(err)
		}
		err = c.List(context.TODO(), &outputBGPAdvertisementList)
		if err != nil {
			log.Fatal(err)
		}
		err = c.List(context.TODO(), &outputL2AdvertisementList)
		if err != nil {
			log.Fatal(err)
		}
		if len(outputAddressPoolList.Items) != tc.outputAddressPoolCount {
			log.Fatalf("TestOnlineMigration(%s): expected to see %d elements but got %d elements of type %s",
				desc, tc.outputAddressPoolCount, len(outputAddressPoolList.Items), "AddressPool")
		}
		if len(outputIPAddressPoolList.Items) != tc.outputIPAddressPoolCount {
			log.Fatalf("TestOnlineMigration(%s): expected to see %d elements but got %d elements of type %s",
				desc, tc.outputIPAddressPoolCount, len(outputIPAddressPoolList.Items), "IPAddressPool")
		}
		if len(outputBGPAdvertisementList.Items) != tc.outputBGPAdvertisementCount {
			log.Fatalf("TestOnlineMigration(%s): expected to see %d elements but got %d elements of type %s",
				desc, tc.outputBGPAdvertisementCount, len(outputBGPAdvertisementList.Items), "BGPAdvertisement")
		}
		if len(outputL2AdvertisementList.Items) != tc.outputL2AdvertisementCount {
			log.Fatalf("TestOnlineMigration(%s): expected to see %d elements but got %d elements of type %s",
				desc, tc.outputL2AdvertisementCount, len(outputL2AdvertisementList.Items), "L2Advertisement")
		}
	}
}

// TODO: These tests would need to be improved, at the moment we are only checking if errors are reported.
func TestObjectCreateAndDelete(t *testing.T) {
	var scheme = runtime.NewScheme()
	err := metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("TestObjectCreateAndDelete: error adding to scheme, err: %q", err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	// Create objects in fake API.
	for _, ap := range validAddressPools0 {
		err := c.Create(context.TODO(), &ap)
		if err != nil {
			t.Fatalf("TestObjectCreateAndDelete: error building fake client, err: %q", err)
		}
	}
	legacyObjects, err := ReadLegacyObjectsFromAPI(c, 0)
	if err != nil {
		t.Fatalf("TestObjectCreateAndDelete: error reading legacy objects from API, err: %q", err)
	}
	if err := legacyObjects.Delete(c); err != nil {
		t.Fatalf("TestObjectCreateAndDelete: error deleting legacy objects from API, err: %q", err)
	}
	if err := legacyObjects.Create(c); err != nil {
		t.Fatalf("TestObjectCreateAndDelete: error creating legacy objects again in API, err: %q", err)
	}
	currentObjects, err := legacyObjects.Convert()
	if err != nil {
		t.Fatalf("TestObjectCreateAndDelete: error converting legacy objects in API, err: %q", err)
	}
	if err := currentObjects.Create(c); err != nil {
		t.Fatalf("TestObjectCreateAndDelete: error creating current objects in API, err: %q", err)
	}
	if err := currentObjects.Delete(c); err != nil {
		t.Fatalf("TestObjectCreateAndDelete: error deleting current objects from API, err: %q", err)
	}
}
