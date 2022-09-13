module github.com/oracle/cluster-api-provider-oci

go 1.16

require (
	github.com/go-logr/logr v1.2.2
	github.com/golang/mock v1.6.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/oracle/oci-go-sdk/v65 v65.18.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.24.2
	k8s.io/apimachinery v0.24.2
	k8s.io/client-go v0.24.2
	k8s.io/component-base v0.24.2
	k8s.io/klog/v2 v2.60.1
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
	sigs.k8s.io/cluster-api v1.2.0
	sigs.k8s.io/cluster-api/test v1.2.0
	sigs.k8s.io/controller-runtime v0.12.3
)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.2.0
