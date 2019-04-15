package reconciler

// ReconcileStageFunc is a function reconciling an entity and returning an error.
type ReconcileStageFunc func() error

// ExecuteStages executes each of the given ReconcileStageFuncs one after the
// other. Execution is aborted on the first error, which will be returned.
func ExecuteStages(funcs ...ReconcileStageFunc) error {
	for _, f := range funcs {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}
