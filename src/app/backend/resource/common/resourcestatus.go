package common

// ResourceStatus provides basic information about resource status on the list
type ResourceStatus struct {
	// Number of resources that are currently in running state
	Running int `json:"running"`

	// Number of resources that are currently in pending state
	Pending int `json:"pending"`

	// Number of resources that are currently in failed state
	Failed int `json:"failed"`

	// Number of resources that are currently in succeeded state
	Succeeded int `json:"succeeded"`
}