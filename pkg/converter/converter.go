package converter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"reflect"

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
	ProtocolBGP       = "bgp"
	ProtocolLayer2    = "layer2"
	metallbAPIGroup   = "metallb.io"
	metallbAPIVersion = "metallb.io/v1beta1"
)

var (
	supportedLegacyGKVVersions = map[string]struct{}{
		"v1beta1": {},
	}
	stdout io.Writer = os.Stdout
)

type Objects interface {
	LegacyObjects | CurrentObjects
	Delete(client.Client) error
	Create(client.Client) error
	Print(targetDirectory string, toJSON bool) error
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
			return fmt.Errorf("cannot delete legacyObject AddressPool '%s', err: %w", ap.Name, err)
		}
	}
	return nil
}

// Create posts all objects to the API.
func (l LegacyObjects) Create(c client.Client) error {
	for _, ap := range l.AddressPoolList.Items {
		err := c.Create(context.TODO(), &ap)
		if err != nil {
			return fmt.Errorf("cannot create legacyObject AddressPool '%s', err: %w", ap.Name, err)
		}
	}
	return nil
}

// Convert converts provided LegacyObjects into current objects.
func (l *LegacyObjects) Convert() (*CurrentObjects, error) {
	apl := l.AddressPoolList
	iapl := &metallbv1beta1.IPAddressPoolList{
		TypeMeta: metav1.TypeMeta{Kind: "IPAddressPoolList", APIVersion: metallbAPIVersion},
	}
	l2al := &metallbv1beta1.L2AdvertisementList{
		TypeMeta: metav1.TypeMeta{Kind: "L2AdvertisementList", APIVersion: metallbAPIVersion},
	}
	bal := &metallbv1beta1.BGPAdvertisementList{
		TypeMeta: metav1.TypeMeta{Kind: "BGPAdvertisementList", APIVersion: metallbAPIVersion},
	}
	for _, ap := range apl.Items {
		iap := metallbv1beta1.IPAddressPool{
			TypeMeta:   metav1.TypeMeta{Kind: "IPAddressPool", APIVersion: metallbAPIVersion},
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
				TypeMeta:   metav1.TypeMeta{Kind: "L2Advertisement", APIVersion: metallbAPIVersion},
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
				name := fmt.Sprintf("%s-bgp-advertisement-%d", ap.Name, i)
				advertisement := legacyBGPAdvertisements[i]
				ba := metallbv1beta1.BGPAdvertisement{
					TypeMeta:   metav1.TypeMeta{Kind: "BGPAdvertisement", APIVersion: metallbAPIVersion},
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

// Print the YAML or JSON representation of the objects either to the  targetDirectory or to stdout if
// targetDirectory == "".
func (l LegacyObjects) Print(targetDirectory string, toJSON bool) error {
	// Skip if there's nothing to do.
	addressPoolList := l.AddressPoolList
	if len(addressPoolList.Items) == 0 {
		return nil
	}
	// Set Kind and APIVersion - the YAML and JSON printers expects those to be set.
	for i := range addressPoolList.Items {
		if addressPoolList.Items[i].Kind == "" {
			addressPoolList.Items[i].Kind = "AddressPool"
		}
		if addressPoolList.Items[i].APIVersion == "" {
			addressPoolList.Items[i].APIVersion = metallbAPIVersion
		}
	}
	// Prepare the output channel and writers.
	outWriter := stdout
	var printer printers.ResourcePrinter = &printers.YAMLPrinter{}
	if toJSON {
		printer = &printers.JSONPrinter{}
	}

	if targetDirectory != "" {
		fileExtension := "yaml"
		if toJSON {
			fileExtension = "json"
		}
		f, err := os.OpenFile(
			path.Join(targetDirectory, fmt.Sprintf("%s.%s", "AddressPool", fileExtension)),
			os.O_RDWR|os.O_CREATE|os.O_TRUNC,
			0644,
		)
		if err != nil {
			return fmt.Errorf("cannot create destination file, err: %w", err)
		}
		defer f.Close()
		outWriter = f
	}
	for _, ap := range addressPoolList.Items {
		printedObj, err := printObj(&ap, printer)
		if err != nil {
			return fmt.Errorf("cannot print object, err: %w\nruntime object: %+v", err, ap)
		}
		fmt.Fprint(outWriter, printedObj)
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
			return fmt.Errorf("cannot delete currentObject IPAddressPool '%s', err: %w", iap.Name, err)
		}
	}
	for _, ba := range c.BGPAdvertisementList.Items {
		err := cl.Delete(context.TODO(), &ba)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("cannot delete currentObject BGPAdvertisement '%s', err: %w", ba.Name, err)
		}
	}
	for _, l2a := range c.L2AdvertisementList.Items {
		err := cl.Delete(context.TODO(), &l2a)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("cannot delete currentObject L2Advertisement '%s', err: %w", l2a.Name, err)
		}
	}
	return nil
}

// Create pods the object to the API.
func (c CurrentObjects) Create(cl client.Client) error {
	for _, iap := range c.IPAddressPoolList.Items {
		err := cl.Create(context.TODO(), &iap)
		if err != nil {
			return fmt.Errorf("cannot create currentObject IPAddressPool '%s', err: %w", iap.Name, err)
		}
	}
	for _, ba := range c.BGPAdvertisementList.Items {
		err := cl.Create(context.TODO(), &ba)
		if err != nil {
			return fmt.Errorf("cannot create currentObject BGPAdvertisement '%s', err: %w", ba.Name, err)
		}
	}
	for _, l2a := range c.L2AdvertisementList.Items {
		err := cl.Create(context.TODO(), &l2a)
		if err != nil {
			return fmt.Errorf("cannot create currentObject L2Advertisement '%s', err: %w", l2a.Name, err)
		}
	}
	return nil
}

// PrintObjects outputs the YAML or JSON representation of the objects (currentObjects or legacyObjects) either to the
// targetDirectory or to stdout if targetDirectory == "".
func (objects CurrentObjects) Print(targetDirectory string, toJSON bool) error {
	outWriter := stdout
	var printer printers.ResourcePrinter = &printers.YAMLPrinter{}
	if toJSON {
		printer = &printers.JSONPrinter{}
	}
	// Iterate over all fields in the struct.
	v := reflect.ValueOf(objects)
	for i := 0; i < v.NumField(); i++ {
		// We expect that each field is a pointer to a List, so it must match runtime.Object.
		currentObject, ok := v.Field(i).Interface().(runtime.Object)
		if !ok {
			return fmt.Errorf("cannot convert field interface to runtime.Object, %s", v.Type().Field(i).Name)
		}
		// Now, reflect the List and get the length of <ListType>.Items. Skip further steps if the list is empty.
		items := reflect.ValueOf(currentObject).Elem().FieldByName("Items")
		if items.Len() == 0 {
			continue
		}
		// Convert a pointer to each list element to a runtime.Object.
		// https://forum.golangbridge.org/t/how-to-cast-interface-to-a-given-interface/13997/15
		var runtimeObjects []runtime.Object
		for j := 0; j < items.Len(); j++ {
			t := reflect.New(items.Index(j).Type())
			t.Elem().Set(items.Index(j))
			runtimeObject := t.Interface().(runtime.Object)
			runtimeObjects = append(runtimeObjects, runtimeObject)
		}
		// We know that we have a least one element, get its type.
		kind := runtimeObjects[0].GetObjectKind().GroupVersionKind().Kind
		if targetDirectory != "" {
			fileExtension := "yaml"
			if toJSON {
				fileExtension = "json"
			}
			f, err := os.OpenFile(
				path.Join(targetDirectory, fmt.Sprintf("%s.%s", kind, fileExtension)),
				os.O_RDWR|os.O_CREATE|os.O_TRUNC,
				0644,
			)
			if err != nil {
				return fmt.Errorf("cannot create destination file, err: %w", err)
			}
			defer f.Close()
			outWriter = f
			// We also must allocate a new printer each time we create a new file (for consistency with "---").
			printer = &printers.YAMLPrinter{}
			if toJSON {
				printer = &printers.JSONPrinter{}
			}
		}
		for _, runtimeObject := range runtimeObjects {
			printedObj, err := printObj(runtimeObject, printer)
			if err != nil {
				return fmt.Errorf("cannot print object, err: %w\nruntime object: %+v", err, runtimeObject)
			}
			fmt.Fprint(outWriter, printedObj)
		}
	}
	return nil
}

// ReadLegacyObjectsFromAPI reads legacy metallb objects from the API.
func ReadLegacyObjectsFromAPI(c client.Client, limit int) (*LegacyObjects, error) {
	if limit < 0 {
		return nil, fmt.Errorf("invalid limit %d", limit)
	}

	addressPoolList := &metallbv1beta1.AddressPoolList{}
	err := c.List(context.Background(), addressPoolList, client.Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to list AddressPools in cluster: %v\n", err)
	}
	// We need the following to accomodate the fake client: https://github.com/kubernetes/client-go/issues/793
	if limit > 0 {
		if len(addressPoolList.Items) > limit {
			addressPoolList.Items = addressPoolList.Items[:limit]
		}
	}
	// Get rid of metadata that we are not interested in.
	for i := range addressPoolList.Items {
		newObjectMeta := metav1.ObjectMeta{
			Name:            addressPoolList.Items[i].Name,
			Namespace:       addressPoolList.Items[i].Namespace,
			Labels:          addressPoolList.Items[i].Labels,
			Annotations:     addressPoolList.Items[i].Annotations,
			OwnerReferences: addressPoolList.Items[i].OwnerReferences,
			Finalizers:      addressPoolList.Items[i].Finalizers,
		}
		addressPoolList.Items[i].ObjectMeta = newObjectMeta
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
			if gkv.Group != metallbAPIGroup {
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

// printObj converts a single runtime.Object to its YAML or JSON representation, depending on the provided
// printers.ResourcePrinter (e.g. *printers.YAMLPrinter or *printers.JSONPrinter).
func printObj(obj runtime.Object, printer printers.ResourcePrinter) (string, error) {
	buf := new(bytes.Buffer)
	err := printer.PrintObj(obj, buf)
	if err != nil {
		return "", fmt.Errorf("issue from printer.PrintObj, err: %w", err)
	}
	return buf.String(), nil
}

// OfflineMigration runs an offline migration. In other words, it reads input from the API or from a source directory
// and either prints it to standard out or a destination directory.
func OfflineMigration(c client.Client, scheme *runtime.Scheme, inDirFlag string, outDirFlag string, jsonFlag bool) error {
	var err error
	var legacyObjects *LegacyObjects
	// Retrieval step.
	if inDirFlag == "" {
		legacyObjects, err = ReadLegacyObjectsFromAPI(c, 0)
		if err != nil {
			return fmt.Errorf("error during retrieval step, err: %w", err)
		}
	} else {
		legacyObjects, err = ReadLegacyObjectsFromDirectory(scheme, inDirFlag)
		if err != nil {
			return fmt.Errorf("error during retrieval step, err: %w", err)
		}
	}
	// Conversion step.
	currentObjects, err := legacyObjects.Convert()
	if err != nil {
		return fmt.Errorf("error during conversion step, err: %w", err)
	}

	// Print step.
	err = currentObjects.Print(outDirFlag, jsonFlag)
	if err != nil {
		return fmt.Errorf("error during print step, err: %w", err)
	}
	return nil
}

// OnlineMigration exectues online migration. It will migrate legacy API resources one by one to their current API
// counterparts.
// Currently, this function cannot roll back. In case of failure, modified objects will be left as is.
func OnlineMigration(c client.Client, scheme *runtime.Scheme, backupDirFlag string, jsonFlag bool) error {
	// Backup as an individual step. This avoids issues with file truncation later down the road and the
	// additional API call shouldn't hurt.
	legacyObjects, err := ReadLegacyObjectsFromAPI(c, 0)
	if err != nil {
		return fmt.Errorf("error during retrieval step, err: %w", err)
	}
	err = legacyObjects.Print(backupDirFlag, jsonFlag)
	if err != nil {
		log.Fatal(err)
	}

	// Now, retrieve, convert, delete and recreate one by one.
	for {
		// Retrieval step.
		legacyObjects, err := ReadLegacyObjectsFromAPI(c, 1)
		if err != nil {
			return fmt.Errorf("error during retrieval step, err: %w", err)
		}
		if len(legacyObjects.AddressPoolList.Items) == 0 {
			break
		}

		log.Printf("migrating AddressPool %s/%s ...", legacyObjects.AddressPoolList.Items[0].Namespace,
			legacyObjects.AddressPoolList.Items[0].Name)

		// Conversion step.
		currentObjects, err := legacyObjects.Convert()
		if err != nil {
			return fmt.Errorf("error during conversion step, err: %w", err)
		}

		// Migration step.
		err = legacyObjects.Delete(c)
		if err != nil {
			return fmt.Errorf("online migration failed during legacy object deletion, err: %w", err)
		}
		err = currentObjects.Create(c)
		if err != nil {
			return fmt.Errorf("online migration failed during current object creation, err: %w", err)
		}
	}
	return nil
}
