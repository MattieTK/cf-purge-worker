package types

import "time"

// WorkerInfo contains details about a Cloudflare Worker
type WorkerInfo struct {
	Name         string
	AccountID    string
	CreatedOn    time.Time
	ModifiedOn   time.Time
	Bindings     []Binding
}

// Binding represents a resource binding in a worker
type Binding struct {
	Type         BindingType
	Name         string
	NamespaceID  string // For KV
	BucketName   string // For R2
	DatabaseID   string // For D1
	DatabaseName string // For D1
	ClassName    string // For Durable Objects
	ScriptName   string // For Durable Objects and Service bindings
	QueueName    string // For Queues
	ConfigID     string // For Hyperdrive
	IndexName    string // For Vectorize
}

// BindingType represents the type of binding
type BindingType string

const (
	BindingTypeKV             BindingType = "kv_namespace"
	BindingTypeR2             BindingType = "r2_bucket"
	BindingTypeD1             BindingType = "d1"
	BindingTypeDurableObject  BindingType = "durable_object_namespace"
	BindingTypeService        BindingType = "service"
	BindingTypeQueue          BindingType = "queue"
	BindingTypeHyperdrive     BindingType = "hyperdrive"
	BindingTypeVectorize      BindingType = "vectorize"
	BindingTypeEnvVar         BindingType = "plain_text"
	BindingTypeSecret         BindingType = "secret_text"
	BindingTypeMTLS           BindingType = "mtls_certificate"
)

// ResourceUsage tracks which workers use a specific resource
type ResourceUsage struct {
	ResourceID   string
	ResourceType BindingType
	ResourceName string
	UsedBy       []string // Worker names
	RiskLevel    RiskLevel
}

// RiskLevel indicates the risk of deleting a resource
type RiskLevel int

const (
	RiskLevelSafe     RiskLevel = iota // Exclusive to this worker
	RiskLevelCaution                   // Used by 1-2 other workers
	RiskLevelDanger                    // Used by 3+ workers
)

// DeletionPlan describes what will be deleted
type DeletionPlan struct {
	Worker            WorkerInfo
	ResourcesToDelete []ResourceUsage
	HasSharedResources bool
	DeleteShared      bool
	DeleteExclusiveOnly bool
}

// DeletionResult tracks the outcome of a deletion operation
type DeletionResult struct {
	Success       bool
	WorkerDeleted bool
	ResourcesDeleted []string
	ResourcesSkipped []string
	Errors        []error
}

// Config holds the application configuration
type Config struct {
	APIKey              string
	AccountID           string
	DryRun              bool
	Force               bool
	ExclusiveOnly       bool
	AutoYes             bool
	Verbose             bool
	Quiet               bool
	JSONOutput          bool
	SkipDependencyCheck bool
}
