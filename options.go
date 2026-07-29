package settle

// Option configures a Settlement.
type Option func(*Settlement)

// WithOptions returns a new Settlement with the given options applied.
// Options are applied in the order they are provided.
//
// Example:
//
//	s := settle.NewSettlement(eur, conv, expenses...).
//	    WithOptions(settle.WithOptimizer(settle.GreedyOptimizer))
//	result, err := s.Settle()
func (s Settlement) WithOptions(opts ...Option) Settlement {
	for _, opt := range opts {
		opt(&s)
	}
	return s
}

// WithOptimizer sets a post-settlement optimizer that runs after the raw
// settlement is computed. The optimizer can reduce the number of
// transactions while preserving the net balance of each participant.
//
// Use the built-in GreedyOptimizer, or provide your own:
//
//	s := settle.NewSettlement(eur, conv, expenses...).
//	    WithOptions(settle.WithOptimizer(settle.GreedyOptimizer))
//
// A custom optimizer is any function matching the Optimizer signature:
//
//	func myOptimizer(result settle.SettlementResult) (settle.SettlementResult, error) {
//	    // ... reduce transactions ...
//	    return optimized, nil
//	}
//
//	s := settle.NewSettlement(eur, conv, expenses...).
//	    WithOptions(settle.WithOptimizer(myOptimizer))
func WithOptimizer(o Optimizer) Option {
	return func(s *Settlement) {
		s.optimizer = o
	}
}
