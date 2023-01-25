package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/andreaskaris/metallb-converter/pkg/converter"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	inDirFlag = flag.String("input-dir", "", "Input directory with legacy style YAML or JSON files."+
		"If empty, read directly from Kubernetes cluster.")
	outDirFlag = flag.String("output-dir", "", "Output directory with new style YAML or JSON files."+
		"If empty, write to stdout.")
	jsonFlag = flag.Bool("json", false, "Write output in JSON format (default YAML)")
)

func main() {
	flag.Parse()

	var legacyObjects *converter.LegacyObjects
	var scheme = runtime.NewScheme()
	err := metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		log.Fatal(err)
	}

	// Retrieval step.
	if *inDirFlag == "" {
		c, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
		if err != nil {
			fmt.Println("failed to create client")
			os.Exit(1)
		}
		legacyObjects, err = converter.ReadLegacyObjectsFromAPI(c)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		legacyObjects, err = converter.ReadLegacyObjectsFromDirectory(scheme, *inDirFlag)
		if err != nil {
			log.Fatal(err)
		}
	}
	// Conversion step.
	currentObjects, err := converter.Convert(legacyObjects)
	if err != nil {
		log.Fatal(err)
	}
	// Print step.
	err = converter.PrintCurrentObjects(currentObjects, *outDirFlag, *jsonFlag)
	if err != nil {
		log.Fatal(err)
	}
}
