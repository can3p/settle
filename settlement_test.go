package settle

import (
	"testing"

	"github.com/Rhymond/go-money"
)

type noopConverter struct{}

func (noopConverter) Convert(a *money.Money, target money.Currency) (*money.Money, error) {
	return money.New(a.Amount(), target.Code), nil
}

func eurCurrency() money.Currency {
	return *money.GetCurrency(money.EUR)
}

func TestSettlement_Settle(t *testing.T) {
	john := StringParticipant("john")
	bill := StringParticipant("bill")
	harry := StringParticipant("harry")
	marry := StringParticipant("marry")
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")

	tests := []struct {
		name     string
		expenses []Expense
		want     map[string]map[string]int64 // from -> to -> amount
		wantErr  bool
	}{
		{
			name: "README example",
			expenses: []Expense{
				NewExpense(john, eur(100), NewEvenLayout(john, bill, harry, marry)),
				NewExpense(bill, eur(100), NewEvenLayout(john, marry)),
			},
			want: map[string]map[string]int64{
				"harry": {"john": 25},
				"john":  {"bill": 25},
				"marry": {"bill": 50, "john": 25},
			},
		},
		{
			name: "single expense even split",
			expenses: []Expense{
				NewExpense(alice, eur(90), NewEvenLayout(alice, bob)),
			},
			want: map[string]map[string]int64{
				"bob": {"alice": 45},
			},
		},
		{
			name: "two opposite equal expenses cancel out",
			expenses: []Expense{
				NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
				NewExpense(bob, eur(100), NewEvenLayout(alice, bob)),
			},
			want: map[string]map[string]int64{},
		},
		{
			name: "payer not in layout",
			expenses: []Expense{
				NewExpense(alice, eur(100), NewEvenLayout(bob, john)),
			},
			want: map[string]map[string]int64{
				"bob":  {"alice": 50},
				"john": {"alice": 50},
			},
		},
		{
			name: "multiple expenses same direction accumulate",
			expenses: []Expense{
				NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
				NewExpense(alice, eur(60), NewEvenLayout(alice, bob)),
			},
			want: map[string]map[string]int64{
				"bob": {"alice": 80},
			},
		},
		{
			name: "reverse transfer smaller than forward",
			expenses: []Expense{
				NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
				NewExpense(bob, eur(20), NewEvenLayout(alice, bob)),
			},
			want: map[string]map[string]int64{
				"bob": {"alice": 40},
			},
		},
		{
			name: "reverse transfer larger than forward",
			expenses: []Expense{
				NewExpense(alice, eur(20), NewEvenLayout(alice, bob)),
				NewExpense(bob, eur(100), NewEvenLayout(alice, bob)),
			},
			want: map[string]map[string]int64{
				"alice": {"bob": 40},
			},
		},
		{
			name: "three people complex",
			expenses: []Expense{
				NewExpense(alice, eur(90), NewEvenLayout(alice, bob, john)),
				NewExpense(bob, eur(60), NewEvenLayout(alice, bob, john)),
			},
			want: map[string]map[string]int64{
				"john": {"alice": 30, "bob": 20},
				"bob":  {"alice": 10},
			},
		},
		{
			name: "weighted shares in settlement",
			expenses: []Expense{
				NewExpense(alice, eur(300), NewShareLayout(
					NewShare(alice, 1), NewShare(bob, 2),
				)),
			},
			want: map[string]map[string]int64{
				"bob": {"alice": 200},
			},
		},
		{
			name:     "no expenses",
			expenses: []Expense{},
			want:     map[string]map[string]int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSettlement(eurCurrency(), noopConverter{}, tt.expenses...)
			result, err := s.Settle()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertSettlement(t, result, tt.want)
		})
	}
}

func TestSettlement_CurrencyConversion(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")

	usd := func(amount int64) *money.Money { return money.New(amount, money.USD) }

	s := NewSettlement(eurCurrency(), noopConverter{},
		NewExpense(alice, usd(100), NewEvenLayout(alice, bob)),
	)

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// noopConverter just carries amount to target currency
	assertSettlement(t, result, map[string]map[string]int64{
		"bob": {"alice": 50},
	})
}

func TestExpense_Split(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	tests := []struct {
		name    string
		expense Expense
		want    map[string]string // from -> to
		amounts map[string]int64  // from -> amount
	}{
		{
			name:    "payer excluded from transfers",
			expense: NewExpense(alice, eur(100), NewEvenLayout(alice, bob, charlie)),
			want:    map[string]string{"bob": "alice", "charlie": "alice"},
			amounts: map[string]int64{"bob": 33, "charlie": 33},
		},
		{
			name:    "payer not in layout",
			expense: NewExpense(alice, eur(100), NewEvenLayout(bob, charlie)),
			want:    map[string]string{"bob": "alice", "charlie": "alice"},
			amounts: map[string]int64{"bob": 50, "charlie": 50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.expense.Split()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, tr := range result {
				fromID := tr.From.ParticipantID()
				expectedTo, ok := tt.want[fromID]
				if !ok {
					t.Errorf("unexpected transfer from %q", fromID)
					continue
				}
				if tr.To.ParticipantID() != expectedTo {
					t.Errorf("from %q: got to=%q, want to=%q", fromID, tr.To.ParticipantID(), expectedTo)
				}
				if tr.Amount.Amount() != tt.amounts[fromID] {
					t.Errorf("from %q: got amount=%d, want %d", fromID, tr.Amount.Amount(), tt.amounts[fromID])
				}
			}
		})
	}
}

func TestSettlement_Deterministic(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	expenses := []Expense{
		NewExpense(alice, eur(100), NewEvenLayout(alice, bob, charlie)),
		NewExpense(bob, eur(60), NewEvenLayout(alice, bob, charlie)),
	}

	s := NewSettlement(eurCurrency(), noopConverter{}, expenses...)

	first, err := s.Settle()
	if err != nil {
		t.Fatal(err)
	}

	// Run multiple times to ensure stable output
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

func TestSettlement_AddExpenses(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	s := NewSettlement(eurCurrency(), noopConverter{},
		NewExpense(alice, eur(90), NewEvenLayout(alice, bob, charlie)),
	)

	s = s.AddExpenses(
		NewExpense(bob, eur(60), NewEvenLayout(alice, bob, charlie)),
	)

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertSettlement(t, result, map[string]map[string]int64{
		"charlie": {"alice": 30, "bob": 20},
		"bob":     {"alice": 10},
	})
}

func TestSettlement_AddExpenses_Multiple(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")

	s := NewSettlement(eurCurrency(), noopConverter{})
	s = s.AddExpenses(
		NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
		NewExpense(alice, eur(60), NewEvenLayout(alice, bob)),
	)

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertSettlement(t, result, map[string]map[string]int64{
		"bob": {"alice": 80},
	})
}

func TestSettlement_AddRepayments(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	// Alice paid 90 for alice, bob, charlie -> bob owes 30, charlie owes 30
	s := NewSettlement(eurCurrency(), noopConverter{},
		NewExpense(alice, eur(90), NewEvenLayout(alice, bob, charlie)),
	)

	// Record that Bob already paid Alice 30
	s = s.AddRepayments(Transfer{
		From:   bob,
		To:     alice,
		Amount: eur(30),
	})

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Bob's debt to Alice is cancelled, only Charlie still owes
	assertSettlement(t, result, map[string]map[string]int64{
		"charlie": {"alice": 30},
	})
}

func TestSettlement_AddTransfers_PartialPayment(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")

	// Bob owes Alice 50
	s := NewSettlement(eurCurrency(), noopConverter{},
		NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
	)

	// Bob already paid 20 to Alice
	s = s.AddRepayments(Transfer{
		From:   bob,
		To:     alice,
		Amount: eur(20),
	})

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Bob still owes 30
	assertSettlement(t, result, map[string]map[string]int64{
		"bob": {"alice": 30},
	})
}

func TestSettlement_AddTransfers_Overpayment(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")

	// Bob owes Alice 50
	s := NewSettlement(eurCurrency(), noopConverter{},
		NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
	)

	// Bob already paid 70 to Alice (overpaid by 20)
	s = s.AddRepayments(Transfer{
		From:   bob,
		To:     alice,
		Amount: eur(70),
	})

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Alice now owes Bob 20
	assertSettlement(t, result, map[string]map[string]int64{
		"alice": {"bob": 20},
	})
}

func TestSettlement_AddExpensesAndTransfers_Combined(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	s := NewSettlement(eurCurrency(), noopConverter{})

	s = s.AddExpenses(
		NewExpense(alice, eur(90), NewEvenLayout(alice, bob, charlie)),
	)

	// Record already-done payments
	s = s.AddRepayments(
		Transfer{From: bob, To: alice, Amount: eur(10)},
		Transfer{From: charlie, To: alice, Amount: eur(30)},
	)

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// bob owed 30, paid 10 -> still owes 20
	// charlie owed 30, paid 30 -> settled
	assertSettlement(t, result, map[string]map[string]int64{
		"bob": {"alice": 20},
	})
}

func TestSettlement_AddTransfers_DoesNotMutateOriginal(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")

	s1 := NewSettlement(eurCurrency(), noopConverter{},
		NewExpense(alice, eur(100), NewEvenLayout(alice, bob)),
	)

	// Adding a repayment returns a new settlement; s1 should be unchanged
	s2 := s1.AddRepayments(Transfer{From: bob, To: alice, Amount: eur(50)})

	r1, err := s1.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := s2.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// s1 still has the original debt
	assertSettlement(t, r1, map[string]map[string]int64{
		"bob": {"alice": 50},
	})
	// s2 is fully settled
	assertSettlement(t, r2, map[string]map[string]int64{})
}

func TestSettlement_AddExpenses_DoesNotMutateOriginal(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")

	s1 := NewSettlement(eurCurrency(), noopConverter{})
	s2 := s1.AddExpenses(NewExpense(alice, eur(100), NewEvenLayout(alice, bob)))

	r1, err := s1.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r2, err := s2.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// s1 has no expenses
	assertSettlement(t, r1, map[string]map[string]int64{})
	// s2 has the added expense
	assertSettlement(t, r2, map[string]map[string]int64{
		"bob": {"alice": 50},
	})
}

func assertSettlement(t *testing.T, got SettlementResult, want map[string]map[string]int64) {
	t.Helper()

	// Count expected transfers
	expectedCount := 0
	for _, tos := range want {
		expectedCount += len(tos)
	}

	if len(got) != expectedCount {
		t.Errorf("got %d transfers, want %d", len(got), expectedCount)
		for _, tr := range got {
			t.Logf("  %s -> %s: %d", tr.From.ParticipantID(), tr.To.ParticipantID(), tr.Amount.Amount())
		}
		return
	}

	for _, tr := range got {
		from := tr.From.ParticipantID()
		to := tr.To.ParticipantID()
		tos, ok := want[from]
		if !ok {
			t.Errorf("unexpected transfer from %q to %q: %d", from, to, tr.Amount.Amount())
			continue
		}
		expectedAmt, ok := tos[to]
		if !ok {
			t.Errorf("unexpected transfer from %q to %q: %d", from, to, tr.Amount.Amount())
			continue
		}
		if tr.Amount.Amount() != expectedAmt {
			t.Errorf("transfer %s -> %s: got %d, want %d", from, to, tr.Amount.Amount(), expectedAmt)
		}
	}
}
