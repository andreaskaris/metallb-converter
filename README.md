# Converter tool for legacy AddressPools to current IPAddressPool format

This is a little converter tool from the legacy AddressPool CRs to the current IPAddressPool CRs. The tool will either
read resources from all namespaces in the cluster, convert them and print the result to standard out or to a provided
target directory. Alternatively, you can also provide an input directory to convert AddressPools stored in YAML files.

## Building the tool

To build the tool, run:
~~~
make build
~~~
> The tool was developed with go 1.18.3, make sure to have the same or a more recent go version.

## Using the tool

If you want to export AddressPools directly from a running cluster, export your KUBECONFIG, then run the tool:
~~~
export KUBECONFIG=<kubeconfig location>
_build/metallb-converter
~~~

If you want to output the generated files to disk, provide an output directory:
~~~
export KUBECONFIG=<kubeconfig location>
tmpdir="$(mktemp -d)"
_build/metallb-converter -output-dir "${tmpdir}"
grep '' "${tmpdir}"/*
~~~

If you want to convert from an input directory containing legacy AddressPool definitions to a target directory:
~~~
_build/metallb-converter -input-dir _examples/ -output-dir _output/
~~~
