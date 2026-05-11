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
	return *money.GetCurrency("EUR")
}

func TestSettlement_Settle(t *testing.T) {
	john := testParticipant("john")
	bill := testParticipant("bill")
	harry := testParticipant("harry")
	marry := testParticipant("marry")
	alice := testParticipant("alice")
	bob := testParticipant("bob")

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
	alice := testParticipant("alice")
	bob := testParticipant("bob")

	usd := func(amount int64) *money.Money { return money.New(amount, "USD") }

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
	alice := testParticipant("alice")
	bob := testParticipant("bob")
	charlie := testParticipant("charlie")

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
	alice := testParticipant("alice")
	bob := testParticipant("bob")
	charlie := testParticipant("charlie")

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
