package nodes

type Usage struct {
	Used  string `json:"used"`
	Total string `json:"total"`
}

type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type Node struct {
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
	Age   string `json:"age"`

	Labels map[string]string `json:"labels"`
	Taints []string          `json:"taints"`

	CPU    Usage `json:"cpu"`
	Memory Usage `json:"memory"`

	Conditions []Condition `json:"conditions"`

	Workloads NodeWorkloads `json:"workloads"`
}

type NodeWorkloads struct {
	Deployments  []Workload `json:"deployments"`
	StatefulSets []Workload `json:"statefulSets"`
	System       []string   `json:"system"`
}

type Workload struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Pods      int    `json:"pods"`
}
