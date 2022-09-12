package constants

// MetaPrefix is the MetaPrefix for labels, annotations, and finalizers of nyamber.
const MetaPrefix = "nyamber.cybozu.io/"

const (
	LabelKeyOwnerNamespace = MetaPrefix + "owner-namespace"
	LabelKeyOwner          = MetaPrefix + "owner"
)

const FinalizerName = MetaPrefix + "finalizer"

// Metadata keys
const (
	// AppNameLabelKey is a label key for application name.
	AppNameLabelKey = "app.kubernetes.io/name"

	// AppComponentLabelKey is a label key for the component.
	AppComponentLabelKey = "app.kubernetes.io/component"

	// AppInstanceLabelKey is a label key for the instance name.
	AppInstanceLabelKey = "app.kubernetes.io/instance"
)

const (
	// AppName is the application name.
	AppName = "nyamber"

	// AppComponentRunner is the component name for runner.
	AppComponentRunner = "runner"
)
