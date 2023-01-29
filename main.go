package main

import (
	"flag"
	"log"

	"github.com/andreaskaris/metallb-converter/pkg/converter"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	jsonFlag      = flag.Bool("json", false, "Write output in JSON format (default YAML).")
	migrationFlag = flag.Bool("online-migration", false, "Trigger an online migration from legacy to new resources.\n"+
		"WARNING: This will reset your BGP sessions, L2 advertisements, and SVC external IPs.\n"+
		"Migration cannot rollback on errors; instead, it will leave resources in a potentially inconsistent state.",
	)
	backupDirFlag = flag.String("backup-dir", "", "Directory that backups of legacy AddressPools will we written to.\n"+
		"Required when migration-flag is set.")
	inDirFlag = flag.String("input-dir", "", "Input directory with legacy style YAML or JSON files.\n"+
		"If empty, read directly from Kubernetes cluster.")
	outDirFlag = flag.String("output-dir", "", "Output directory with new style YAML or JSON files.\n"+
		"If empty, write to stdout.")
)

func main() {
	flag.Parse()

	var c client.Client
	var scheme = runtime.NewScheme()
	err := metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		log.Fatal(err)
	}

	// Verify parameters.
	if *migrationFlag {
		if *inDirFlag != "" || *outDirFlag != "" || *jsonFlag {
			log.Fatal("no other option may be set if online-migration is requested")
		}
		if *backupDirFlag == "" {
			log.Fatal("you must set a backup directory when migrating resources")
		}
	} else {
		if *backupDirFlag != "" {
			log.Fatal("backup-dir is only allowed for migrations")
		}
	}

	// Set up the client.
	if *inDirFlag == "" {
		conf, err := config.GetConfig()
		if err != nil {
			log.Fatalf("error getting kubernetes configuration, did you export KUBECONFIG? Received error: %q", err)
		}
		c, err = client.New(conf, client.Options{Scheme: scheme})
		if err != nil {
			log.Fatal(err)
		}
	}

	// Either print to stdout or to directory ..o
	if !*migrationFlag {
		err = converter.OfflineMigration(c, scheme, *inDirFlag, *outDirFlag, *jsonFlag)
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	// or migrate the API objects directly.
	err = converter.OnlineMigration(c, scheme, *backupDirFlag, *jsonFlag)
	if err != nil {
		log.Fatal(err)
	}
}
