package api

// Status mirrors the Scala enum at orchard-core/.../model/Status.scala.
type Status string

const (
	StatusPending       Status = "pending"
	StatusActivating    Status = "activating"
	StatusRunning       Status = "running"
	StatusFinished      Status = "finished"
	StatusFailed        Status = "failed"
	StatusCascadeFailed Status = "cascade_failed"
	StatusCanceling     Status = "canceling"
	StatusCanceled      Status = "canceled"
	StatusDeactivating  Status = "deactivating"
	StatusShuttingDown  Status = "shutting_down"
	StatusTimeout       Status = "timeout"
	StatusDeleted       Status = "deleted"
)

// AllStatuses lists every defined status in the order orchard's enum declares
// them.
var AllStatuses = []Status{
	StatusPending,
	StatusActivating,
	StatusRunning,
	StatusFinished,
	StatusFailed,
	StatusCascadeFailed,
	StatusCanceling,
	StatusCanceled,
	StatusDeactivating,
	StatusShuttingDown,
	StatusTimeout,
	StatusDeleted,
}

// IsTerminal reports whether the status indicates the entity has stopped
// progressing on its own and only manual intervention or already-final state
// remains.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusFinished, StatusFailed, StatusCascadeFailed,
		StatusCanceled, StatusTimeout, StatusDeleted:
		return true
	}
	return false
}

// IsActive reports whether the status indicates ongoing work that warrants
// fast polling.
func (s Status) IsActive() bool {
	switch s {
	case StatusActivating, StatusRunning, StatusCanceling,
		StatusDeactivating, StatusShuttingDown:
		return true
	}
	return false
}

func (s Status) String() string { return string(s) }
