package update

import (
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/resource"
)

type ContainerSpecs struct {
	Kind           ReleaseKind
	ContainerSpecs map[flux.ResourceID][]ContainerUpdate
}

func (s ContainerSpecs) CalculateRelease(rc ReleaseContext, logger log.Logger) ([]*ControllerUpdate, Result, error) {
	results := Result{}

	// Collect resources we have a spec on. We then only query further
	// information such as containers for these.
	var rids []flux.ResourceID
	for rid := range s.ContainerSpecs {
		rids = append(rids, rid)
	}

	all, err := rc.SelectServices(results, []ControllerFilter{&IncludeFilter{IDs: rids}}, nil)
	if err != nil {
		return nil, results, err
	}

	var updates []*ControllerUpdate
	for _, u := range all {
		cs, err := u.Controller.ContainersOrError()
		if err != nil {
			results[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}
		// All containers of a controller
		containers := map[string]resource.Container{}
		for _, spec := range cs {
			containers[spec.Name] = spec
		}

		// Go through specs and collect updates. We make sure here
		// to do this in order of the container specs supplied.
		var containerUpdates []ContainerUpdate
		for _, spec := range s.ContainerSpecs[u.ResourceID] {
			container, ok := containers[spec.Container]
			if !ok {
				results[u.ResourceID] = ControllerResult{
					Status: ReleaseStatusFailed,
					Error:  "container not found",
				}
				break // go to next controller
			}

			if container.Image != spec.Current {
				results[u.ResourceID] = ControllerResult{
					Status: ReleaseStatusFailed,
					Error:  "unexpected container image tag",
				}
				break // go to next controller
			}
			containerUpdates = append(u.Updates, spec)
		}

		if _, ok := results[u.ResourceID]; !ok {
			u.Updates = containerUpdates
			updates = append(updates, u)
			results[u.ResourceID] = ControllerResult{
				Status:       ReleaseStatusSuccess,
				PerContainer: u.Updates,
			}
		}
	}

	return updates, results, nil
}

func (s ContainerSpecs) ReleaseKind() ReleaseKind {
	return s.Kind
}

func (s ContainerSpecs) ReleaseType() ReleaseType {
	return "containers"
}

func (s ContainerSpecs) CommitMessage(result Result) string {
	return "Container release"
}
