package constants

// MetaPrefix is the MetaPrefix for labels, annotations, and finalizers of nyamber.
const MetaPrefix = "nyamber.cybozu.io/"

const (
	LabelKeyOwnerNamespace = MetaPrefix + "owner-namespace"
	LabelKeyOwner          = MetaPrefix + "owner"
)

const FinalizerName = MetaPrefix + "finalizer"
