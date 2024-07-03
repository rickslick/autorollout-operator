package consts

const (
	ReasonRolloutRestartFailed      = "RolloutRestartFailed"
	ReasonRolloutRestartTriggered   = "RolloutRestartTriggered"
	ReasonRolloutRestartUnsupported = "RolloutRestartUnsupported"
	ReasonAnnotationSucceeded       = "AnnotationAdditionSucceeded"
	ReasonAnnotationFailed          = "AnnotationAdditionFailed"
)
const (
	DEFAULT_FLIPPER_INTERVAL = 10
)

const (
	AnnotationFlipperRestartedAt = "flipper.ricktech.io/restartedAt"
	RolloutRestartAnnotation     = "kubectl.kubernetes.io/restartedAt"
	RolloutManagedBy             = "flipper.ricktech.io/managedBy"
	rolloutIntervalGroupName     = "flipper.ricktech.io/IntervalGroup"
)

const (
	ErrorUnsupportedKind = "unsupported Kind %v"
)
