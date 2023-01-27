package converter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ProtocolBGP is a string representation of the BGP protocol.
	ProtocolBGP             = "bgp"
	ProtocolLayer2          = "layer2"
	supportedlegacyGKVGroup = "metallb.io"
)

var (
	supportedLegacyGKVVersions = map[string]struct{}{
		"v1beta1": struct{}{},
	}
	stdout io.Writer = os.Stdout
)

type Objects interface {
	LegacyObjects | CurrentObjects
	Delete(client.Client) error
	Create(client.Client) error
}

// LegacyObjects holds metallb legacy objects that shall be converted to the new format.
type LegacyObjects struct {
	AddressPoolList *metallbv1beta1.AddressPoolList
}

// Delete deletes all objects that belong to this object from the API.
func (l LegacyObjects) Delete(c client.Client) error {
	for _, ap := range l.AddressPoolList.Items {
		err := c.Delete(context.TODO(), &ap)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// Create posts all objects to the API.
func (l LegacyObjects) Create(c client.Client) error {
	for _, ap := range l.AddressPoolList.Items {
		err := c.Create(context.TODO(), &ap)
		if err != nil {
			return err
		}
	}
	return nil
}

// CurrentObjects holds metallb current objects after conversion from the legacy format.
type CurrentObjects struct {
	IPAddressPoolList    *metallbv1beta1.IPAddressPoolList
	L2AdvertisementList  *metallbv1beta1.L2AdvertisementList
	BGPAdvertisementList *metallbv1beta1.BGPAdvertisementList
}

// Delete deletes all instances from the API if they exist.
func (c CurrentObjects) Delete(cl client.Client) error {
	for _, iap := range c.IPAddressPoolList.Items {
		err := cl.Delete(context.TODO(), &iap)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	for _, ba := range c.BGPAdvertisementList.Items {
		err := cl.Delete(context.TODO(), &ba)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	for _, l2a := range c.L2AdvertisementList.Items {
		err := cl.Delete(context.TODO(), &l2a)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// Create pods the object to the API.
func (c CurrentObjects) Create(cl client.Client) error {
	for _, iap := range c.IPAddressPoolList.Items {
		err := cl.Create(context.TODO(), &iap)
		if err != nil {
			return err
		}
	}
	for _, ba := range c.BGPAdvertisementList.Items {
		err := cl.Create(context.TODO(), &ba)
		if err != nil {
			return err
		}
	}
	for _, l2a := range c.L2AdvertisementList.Items {
		err := cl.Create(context.TODO(), &l2a)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadLegacyObjectsFromAPI reads legacy metallb objects from the API.
func ReadLegacyObjectsFromAPI(c client.Client) (*LegacyObjects, error) {
	addressPoolList := &metallbv1beta1.AddressPoolList{}
	err := c.List(context.Background(), addressPoolList)
	if err != nil {
		return nil, fmt.Errorf("failed to list AddressPools in cluster: %v\n", err)
	}
	return &LegacyObjects{
		AddressPoolList: addressPoolList,
	}, nil
}

// ReadLegacyObjectsFromAPI reads legacy metallb objects from a given directory.
// A lot of the logic was derived from:
// https://medium.com/@harshjniitr/reading-and-writing-k8s-resource-as-yaml-in-golang-81dc8c7ea800
func ReadLegacyObjectsFromDirectory(scheme *runtime.Scheme, dir string) (*LegacyObjects, error) {
	addressPoolList := &metallbv1beta1.AddressPoolList{}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not read legacy objects from directory, err: %q", err)
	}
	for _, file := range files {
		decode := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode
		fileContent, err := os.ReadFile(path.Join(dir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("could not read legacy objects from directory, err: %q", err)
		}
		elements := bytes.Split(fileContent, []byte("\n---"))
		for _, element := range elements {
			obj, gkv, err := decode(element, nil, nil)
			if err != nil {
				return nil, fmt.Errorf("could not read legacy objects from directory, err: %q", err)
			}
			if gkv.Group != supportedlegacyGKVGroup {
				return nil, fmt.Errorf("could not read legacy objects from directory, invalid gkv.Group %q", gkv.Group)
			}
			if _, ok := supportedLegacyGKVVersions[gkv.Version]; !ok {
				return nil, fmt.Errorf("could not read legacy objects from directory, invalid gkv.Version %q", gkv.Version)
			}
			switch gkv.Kind {
			case "AddressPool":
				ap := obj.(*metallbv1beta1.AddressPool)
				addressPoolList.Items = append(addressPoolList.Items, *ap)
			case "AddressPoolList":
				apl := obj.(*metallbv1beta1.AddressPoolList)
				addressPoolList.Items = append(addressPoolList.Items, apl.Items...)
			default:
				return nil, fmt.Errorf("could not read legacy objects from directory, unsupported GKV: %s", gkv.Kind)
			}
		}
	}
	return &LegacyObjects{
		AddressPoolList: addressPoolList,
	}, nil
}

// Convert converts provided LegacyObjects into current objects.
func Convert(legacyObjects *LegacyObjects) (*CurrentObjects, error) {
	apl := legacyObjects.AddressPoolList
	iapl := &metallbv1beta1.IPAddressPoolList{
		TypeMeta: metav1.TypeMeta{Kind: "IPAddressPoolList", APIVersion: "metallb.io/v1beta1"},
	}
	l2al := &metallbv1beta1.L2AdvertisementList{
		TypeMeta: metav1.TypeMeta{Kind: "L2AdvertisementList", APIVersion: "metallb.io/v1beta1"},
	}
	bal := &metallbv1beta1.BGPAdvertisementList{
		TypeMeta: metav1.TypeMeta{Kind: "BGPAdvertisementList", APIVersion: "metallb.io/v1beta1"},
	}
	for _, ap := range apl.Items {
		iap := metallbv1beta1.IPAddressPool{
			TypeMeta:   metav1.TypeMeta{Kind: "IPAddressPool", APIVersion: "metallb.io/v1beta1"},
			ObjectMeta: metav1.ObjectMeta{Name: ap.ObjectMeta.Name, Namespace: ap.ObjectMeta.Namespace},
			Spec: metallbv1beta1.IPAddressPoolSpec{
				Addresses:  ap.Spec.Addresses,
				AutoAssign: ap.Spec.AutoAssign,
			},
			Status: metallbv1beta1.IPAddressPoolStatus{},
		}
		iapl.Items = append(iapl.Items, iap)

		if ap.Spec.Protocol == ProtocolLayer2 {
			name := fmt.Sprintf("%s-l2-advertisement", ap.Name)
			l2a := metallbv1beta1.L2Advertisement{
				TypeMeta:   metav1.TypeMeta{Kind: "L2Advertisement", APIVersion: "metallb.io/v1beta1"},
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ap.Namespace},
				Spec: metallbv1beta1.L2AdvertisementSpec{
					IPAddressPools: []string{ap.Name},
				},
			}
			l2al.Items = append(l2al.Items, l2a)
		} else if ap.Spec.Protocol == ProtocolBGP {
			// If the optional BGPAdvertisements are not set, create a dummy advertisement. This allows us to iterate
			// over the legacyBGPAdvertisements and create new BGPAdvertisement CRs instead. Because we are appending
			// to the list, we must deep copy the existing legacy advertisements first.
			legacyBGPAdvertisements := ap.Spec.DeepCopy().BGPAdvertisements
			if len(legacyBGPAdvertisements) == 0 {
				legacyBGPAdvertisements = append(legacyBGPAdvertisements, metallbv1beta1.LegacyBgpAdvertisement{})
			}
			for i := 0; i < len(legacyBGPAdvertisements); i++ {
				name := fmt.Sprintf("%s-l2-advertisement-%d", ap.Name, i)
				advertisement := legacyBGPAdvertisements[i]
				ba := metallbv1beta1.BGPAdvertisement{
					TypeMeta:   metav1.TypeMeta{Kind: "BGPAdvertisement", APIVersion: "metallb.io/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ap.Namespace},
					Spec: metallbv1beta1.BGPAdvertisementSpec{
						AggregationLength:   advertisement.AggregationLength,
						AggregationLengthV6: advertisement.AggregationLengthV6,
						LocalPref:           advertisement.LocalPref,
						Communities:         advertisement.Communities,
						IPAddressPools:      []string{ap.Name},
					},
					Status: metallbv1beta1.BGPAdvertisementStatus{},
				}
				bal.Items = append(bal.Items, ba)
			}
		} else {
			return nil, fmt.Errorf("unsupported Spec.Protocol for AddressPool, %v", ap)
		}
	}
	return &CurrentObjects{
		IPAddressPoolList:    iapl,
		L2AdvertisementList:  l2al,
		BGPAdvertisementList: bal,
	}, nil
}

// PrintObjects outputs the YAML or JSON representation of the objects (currentObjects or legacyObjects) either to the
// targetDirectory or to stdout if targetDirectory == "".
func PrintObjects[T Objects](objects *T, targetDirectory string, toJSON bool) error {
	var printer printers.ResourcePrinter = &printers.YAMLPrinter{}
	if toJSON {
		printer = &printers.JSONPrinter{}
	}
	// Iterate over all fields in the struct.
	v := reflect.ValueOf(*objects)
	for i := 0; i < v.NumField(); i++ {
		// We expect that each field is a pointer to a List, so it must match runtime.Object.
		currentObject, ok := v.Field(i).Interface().(runtime.Object)
		if !ok {
			return fmt.Errorf("cannot convert field interface to runtime.Object, %s", v.Type().Field(i).Name)
		}
		// Now, reflect the List and get the length of <ListType>.Items. Skip further steps if the list is empty.
		if reflect.ValueOf(currentObject).Elem().FieldByName("Items").Len() == 0 {
			continue
		}
		kind := currentObject.GetObjectKind().GroupVersionKind().Kind
		outWriter := stdout
		if targetDirectory != "" {
			f, err := os.OpenFile(
				path.Join(targetDirectory, fmt.Sprintf("%s.yaml", kind)),
				os.O_RDWR|os.O_CREATE|os.O_TRUNC,
				0644,
			)
			if err != nil {
				return err
			}
			defer f.Close()
			outWriter = f
		} else {
			if i > 0 {
				fmt.Fprint(outWriter, "\n")
			}
		}
		printedObj, err := printObj(currentObject, printer)
		if err != nil {
			return err
		}
		fmt.Fprint(outWriter, strings.Trim(printedObj, "\n"))
	}
	return nil
}

// printObj converts a single runtime.Object to its YAML or JSON representation, depending on the provided
// printers.ResourcePrinter (e.g. *printers.YAMLPrinter or *printers.JSONPrinter).
func printObj(obj runtime.Object, printer printers.ResourcePrinter) (string, error) {
	buf := new(bytes.Buffer)
	err := printer.PrintObj(obj, buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// OnlineMigration receives a set of Objects to delete and a set of Objects to create. It will delete all Objects to
// delete one by one. Then, it will create the Objects to create.
// Currently, the function cannot rollback on errors; instead, it will leave resources in a potentially inconsistent
// state.
func OnlineMigration[T Objects, U Objects](c client.Client, toDelete T, toCreate U) error {
	err := toDelete.Delete(c)
	if err != nil {
		return err
	}
	return toCreate.Create(c)
}
