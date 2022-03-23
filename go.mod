module github.com/oracle/cluster-api-provider-oci

go 1.16

require (
	github.com/go-logr/logr v1.2.2
	github.com/golang/mock v1.6.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/oracle/oci-go-sdk/v63 v63.0.0
	github.com/pkg/errors v0.9.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/cluster-api v1.0.1-0.20211111175208-4cc2fce2111a
	sigs.k8s.io/cluster-api/test v1.1.0-beta.2.0.20220225180551-56d99d7bca51
	sigs.k8s.io/controller-runtime v0.11.1
)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.0.1-0.20211111175208-4cc2fce2111a
