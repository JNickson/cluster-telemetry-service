package pods

type Pod struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Node      string `json:"node"`

	Phase    string `json:"phase"`
	Ready    bool   `json:"ready"`
	Restarts int32  `json:"restarts"`
	Age      string `json:"age"`

	Containers []Container `json:"containers"`
	Logs       string      `json:"logs,omitempty"`
}

type Container struct {
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}
