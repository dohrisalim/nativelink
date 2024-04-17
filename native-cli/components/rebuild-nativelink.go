package components

import (
	_ "embed"
	"fmt"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type RebuildNativeLink struct {
	Dependencies []pulumi.Resource
}

// These are vendored yaml files which we don't port to Pulumi so that we can
// potentially adjust/reuse them in more generic contexts. We embed them in the
// executable to keep the cli portable.
//
//go:embed embedded/rebuild-nativelink.yaml
var rebuildNativeLinkYaml string

//go:embed embedded/trigger.yaml
var triggerYaml string

// Install installs a Tekton Task, Pipeline and EventListener and some
// supporting resources which ultimately allow querying the cluster at a Gateway
// named `eventlistener` with requests like so:
//
// ```
// EVENTLISTENER=$(kubectl get gtw eventlistener -o=jsonpath='{.status.addresses[0].value}')
//
// # If imageNameOverride and imageTagOverride are unset, they default to:
// # $(nix eval <flakeOutput>.imageName --raw)
// # $(nix eval <flakeOutput>.imageTag --raw)
//
//	curl -v \
//	    -H 'content-Type: application/json' \
//	    -d '{
//	        "flakeOutput": "./src_root#image",
//	        "imageNameOverride": "nativelink",
//	        "imageTagOverride": "local"
//	    }' \
//	    http://${EVENTLISTENER}:8080
//
// ```
//
// This pipeline only works with the specific local setup for the NativeLink
// development cluster. The Task makes use of the double-pipe through volumes
// `host -> kind -> K8s` to reuse the host's nix store and local nativelink git
// repository. It then pushes the container image to the container registry
// which previous infrastructure setups configured to pass through from host to
// the cluster. The result is that these Pipelines can complete in <15sec as
// opposed to ~10min without these optimizations.
//
// WARNING: At the moment the Task makes use of `SYS_ADMIN` privilege escalation
// to interact with the host's nix socket and the kind node's container daemon.
func (component *RebuildNativeLink) Install(
	ctx *pulumi.Context,
	name string,
) ([]pulumi.Resource, error) {
	rebuildNativeLink, err := yaml.NewConfigGroup(
		ctx,
		name,
		&yaml.ConfigGroupArgs{
			YAML: []string{rebuildNativeLinkYaml},
		},
		pulumi.DependsOn(component.Dependencies),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPulumi, err)
	}

	ciTrigger, err := yaml.NewConfigGroup(
		ctx,
		name+"-triggers",
		&yaml.ConfigGroupArgs{
			YAML: []string{triggerYaml},
		},
		pulumi.DependsOn(component.Dependencies),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPulumi, err)
	}

	return []pulumi.Resource{rebuildNativeLink, ciTrigger}, nil
}
