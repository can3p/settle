package settle

import (
	"testing"

	"github.com/Rhymond/go-money"
	"github.com/shopspring/decimal"
)

// netBalances computes the net balance for each participant from a settlement
// result. A positive balance means the participant is owed money; a negative
// balance means they owe money.
func netBalances(t *testing.T, result SettlementResult) map[string]int64 {
	t.Helper()
	balances := map[string]int64{}
	for _, tr := range result {
		balances[tr.From.ParticipantID()] -= tr.Amount.Amount()
		balances[tr.To.ParticipantID()] += tr.Amount.Amount()
	}
	return balances
}

func TestSettlement_GreedyOptimization(t *testing.T) {
	john := StringParticipant("john")
	bill := StringParticipant("bill")
	harry := StringParticipant("harry")
	marry := StringParticipant("marry")
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")
	dave := StringParticipant("dave")

	tests := []struct {
		name          string
		expenses      []Expense
		repayments    []Transfer
		wantOptimized map[string]map[string]int64
	}{
		{
			name: "README example reduces from 4 to 2 transfers",
			expenses: []Expense{
				NewExpense(john, eur(100), NewEvenLayout(john, bill, harry, marry)),
				NewExpense(bill, eur(100), NewEvenLayout(john, marry)),
			},
			wantOptimized: map[string]map[string]int64{
				"harry": {"john": 25},
				"marry": {"bill": 75},
			},
		},
		{
			name: "three people complex reduces from 3 to 2 transfers",
			expenses: []Expense{
				NewExpense(alice, eur(90), NewEvenLayout(alice, bob, john)),
				NewExpense(bob, eur(60), NewEvenLayout(alice, bob, john)),
			},
			wantOptimized: map[string]map[string]int64{
				"john": {"alice": 40, "bob": 10},
			},
		},
		{
			name: "single transfer unchanged",
			expenses: []Expense{
				NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
			},
			wantOptimized: map[string]map[string]int64{
				"bob": {"alice": 50},
			},
		},
		{
			name:          "no expenses",
			expenses:      []Expense{},
			wantOptimized: map[string]map[string]int64{},
		},
		{
			name: "opposite expenses cancel out",
			expenses: []Expense{
				NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
				NewExpense(bob, eur(100), NewEvenLayout(alice, bob)),
			},
			wantOptimized: map[string]map[string]int64{},
		},
		{
			name: "four people reduces from 4 to 2 transfers",
			expenses: []Expense{
				NewExpense(alice, eur(100), NewEvenLayout(alice, bob, charlie, dave)),
				NewExpense(bob, eur(100), NewEvenLayout(alice, bob, charlie, dave)),
			},
			wantOptimized: map[string]map[string]int64{
				"charlie": {"bob": 50},
				"dave":    {"alice": 50},
			},
		},
		{
			name: "with repayments reduces from 5 to 3 transfers",
			expenses: []Expense{
				NewExpense(alice, eur(100), NewEvenLayout(alice, bob, charlie, dave)),
				NewExpense(bob, eur(100), NewEvenLayout(alice, bob, charlie, dave)),
			},
			repayments: []Transfer{
				{From: alice, To: bob, Amount: eur(25)},
			},
			wantOptimized: map[string]map[string]int64{
				"charlie": {"alice": 50},
				"dave":    {"alice": 25, "bob": 25},
			},
		},
		{
			name: "weighted shares with optimization",
			expenses: []Expense{
				NewExpense(alice, eur(300), NewShareLayout(
					NewShare(alice, 1), NewShare(bob, 2),
				)),
			},
			wantOptimized: map[string]map[string]int64{
				"bob": {"alice": 200},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Raw settlement (without optimization)
			raw := NewSettlement(eurCurrency(), noopConverter{}, tt.expenses...).
				AddRepayments(tt.repayments...)
			rawResult, err := raw.Settle()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Optimized settlement
			opt := NewSettlement(eurCurrency(), noopConverter{}, tt.expenses...).
				AddRepayments(tt.repayments...).
				WithOptions(WithOptimizer(GreedyOptimizer))
			optResult, err := opt.Settle()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the optimized result matches the expected
			assertSettlement(t, optResult, tt.wantOptimized)

			// Verify the number of transfers is reduced or same
			if len(optResult) > len(rawResult) {
				t.Errorf("optimized result has %d transfers, more than raw %d",
					len(optResult), len(rawResult))
			}

			// Verify net balances are preserved
			rawBalances := netBalances(t, rawResult)
			optBalances := netBalances(t, optResult)
			if len(rawBalances) != len(optBalances) {
				t.Errorf("net balances have different participant counts: raw=%d, opt=%d",
					len(rawBalances), len(optBalances))
			}
			for id, rawBal := range rawBalances {
				optBal, ok := optBalances[id]
				if !ok {
					t.Errorf("participant %q missing from optimized balances", id)
					continue
				}
				if rawBal != optBal {
					t.Errorf("participant %q: raw balance=%d, opt balance=%d", id, rawBal, optBal)
				}
			}
		})
	}
}

func TestSettlement_GreedyOptimization_Deterministic(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")
	dave := StringParticipant("dave")

	// This case has ties (Alice and Bob both +50, Charlie and Dave both -50),
	// which tests that tie-breaking is deterministic.
	expenses := []Expense{
		NewExpense(alice, eur(100), NewEvenLayout(alice, bob, charlie, dave)),
		NewExpense(bob, eur(100), NewEvenLayout(alice, bob, charlie, dave)),
	}

	s := NewSettlement(eurCurrency(), noopConverter{}, expenses...).
		WithOptions(WithOptimizer(GreedyOptimizer))

	first, err := s.Settle()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 20; i++ {
		result, err := s.Settle()
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != len(first) {
			t.Fatalf("iteration %d: got %d transfers, want %d", i, len(result), len(first))
		}
		for j := range result {
			if result[j].From.ParticipantID() != first[j].From.ParticipantID() ||
				result[j].To.ParticipantID() != first[j].To.ParticipantID() ||
				result[j].Amount.Amount() != first[j].Amount.Amount() {
				t.Fatalf("iteration %d: result differs at index %d", i, j)
			}
		}
	}
}

func TestSettlement_GreedyOptimization_BackwardCompatible(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	expenses := []Expense{
		NewExpense(alice, eur(90), NewEvenLayout(alice, bob, charlie)),
		NewExpense(bob, eur(60), NewEvenLayout(alice, bob, charlie)),
	}

	// Without optimizer — should produce the same result as before
	s := NewSettlement(eurCurrency(), noopConverter{}, expenses...)
	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertSettlement(t, result, map[string]map[string]int64{
		"charlie": {"alice": 30, "bob": 20},
		"bob":     {"alice": 10},
	})
}

func TestSettlement_GreedyOptimization_WithCurrencyConversion(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.EUR, decimal.NewFromFloat(0.9))
	conv.AddRate(money.JPY, money.EUR, decimal.NewFromFloat(0.006))

	s := NewSettlement(eurCurrency(), conv,
		NewExpense(alice, usd(100_00), NewEvenLayout(alice, bob, charlie)),
		NewExpense(bob, jpy(10000), NewEvenLayout(alice, bob, charlie)),
	).WithOptions(WithOptimizer(GreedyOptimizer))

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without optimization: bob->alice:1000, charlie->alice:3000, charlie->bob:2000 (3 transfers)
	// Net balances: alice=+4000, bob=+1000, charlie=-5000
	// Optimized: charlie->alice:4000, charlie->bob:1000 (2 transfers)
	assertSettlement(t, result, map[string]map[string]int64{
		"charlie": {"alice": 4000, "bob": 1000},
	})

	if len(result) != 2 {
		t.Errorf("expected 2 transfers, got %d", len(result))
	}
}

func TestSettlement_GreedyOptimization_WithOptionsAndAddExpenses(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	// Set the optimizer first, then add expenses — optimizer should be carried over
	s := NewSettlement(eurCurrency(), noopConverter{}).
		WithOptions(WithOptimizer(GreedyOptimizer)).
		AddExpenses(
			NewExpense(alice, eur(90), NewEvenLayout(alice, bob, charlie)),
			NewExpense(bob, eur(60), NewEvenLayout(alice, bob, charlie)),
		)

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without optimization: 3 transfers (bob->alice:10, charlie->alice:30, charlie->bob:20)
	// With optimization: 2 transfers (charlie->alice:40, charlie->bob:10)
	assertSettlement(t, result, map[string]map[string]int64{
		"charlie": {"alice": 40, "bob": 10},
	})

	if len(result) != 2 {
		t.Errorf("expected 2 transfers, got %d", len(result))
	}
}

func TestOptimizeGreedy_Directly(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	tests := []struct {
		name  string
		input SettlementResult
		want  map[string]map[string]int64
	}{
		{
			name: "reduces three transfers to two",
			input: SettlementResult{
				{From: bob, To: alice, Amount: eur(10)},
				{From: charlie, To: alice, Amount: eur(30)},
				{From: charlie, To: bob, Amount: eur(20)},
			},
			want: map[string]map[string]int64{
				"charlie": {"alice": 40, "bob": 10},
			},
		},
		{
			name:  "empty result",
			input: SettlementResult{},
			want:  map[string]map[string]int64{},
		},
		{
			name: "single transfer unchanged",
			input: SettlementResult{
				{From: bob, To: alice, Amount: eur(50)},
			},
			want: map[string]map[string]int64{
				"bob": {"alice": 50},
			},
		},
		{
			name: "all transfers cancel out",
			input: SettlementResult{
				{From: bob, To: alice, Amount: eur(50)},
				{From: alice, To: bob, Amount: eur(50)},
			},
			want: map[string]map[string]int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GreedyOptimizer(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertSettlement(t, result, tt.want)
		})
	}
}

func TestSettlement_WithOptimizer_Custom(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	expenses := []Expense{
		NewExpense(alice, eur(90), NewEvenLayout(alice, bob, charlie)),
		NewExpense(bob, eur(60), NewEvenLayout(alice, bob, charlie)),
	}

	// A custom optimizer that simply reverses the direction of every transfer.
	// This is a no-op for net balances but proves that any Optimizer works.
	reverseOptimizer := func(result SettlementResult) (SettlementResult, error) {
		reversed := make(SettlementResult, len(result))
		for i, tr := range result {
			reversed[i] = Transfer{
				From:   tr.To,
				To:     tr.From,
				Amount: tr.Amount,
			}
		}
		return reversed, nil
	}

	s := NewSettlement(eurCurrency(), noopConverter{}, expenses...).
		WithOptions(WithOptimizer(reverseOptimizer))

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Raw result: bob->alice:10, charlie->alice:30, charlie->bob:20
	// Reversed:   alice->bob:10, alice->charlie:30, bob->charlie:20
	assertSettlement(t, result, map[string]map[string]int64{
		"alice": {"bob": 10, "charlie": 30},
		"bob":   {"charlie": 20},
	})
}
