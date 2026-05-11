package settle

import (
	"testing"

	"github.com/Rhymond/go-money"
)

type testParticipant string

func (t testParticipant) ParticipantID() string { return string(t) }

func eur(amount int64) *money.Money { return money.New(amount, "EUR") }

func assertParts(t *testing.T, got []ParticipantExpence, want map[string]int64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d parts, want %d", len(got), len(want))
	}

	for _, p := range got {
		id := p.participant.ParticipantID()
		expected, ok := want[id]
		if !ok {
			t.Errorf("unexpected participant %q", id)
			continue
		}
		if p.amount.Amount() != expected {
			t.Errorf("participant %q: got %d, want %d", id, p.amount.Amount(), expected)
		}
	}
}

func TestShareLayout_Split(t *testing.T) {
	alice := testParticipant("alice")
	bob := testParticipant("bob")
	charlie := testParticipant("charlie")

	tests := []struct {
		name    string
		shares  []share
		amount  *money.Money
		want    map[string]int64
		wantErr bool
	}{
		{
			name:   "even split, divisible",
			shares: []share{NewShare(alice, 1), NewShare(bob, 1)},
			amount: eur(100),
			want:   map[string]int64{"alice": 50, "bob": 50},
		},
		{
			name:   "even split, indivisible remainder goes to first",
			shares: []share{NewShare(alice, 1), NewShare(bob, 1), NewShare(charlie, 1)},
			amount: eur(100),
			want:   map[string]int64{"alice": 34, "bob": 33, "charlie": 33},
		},
		{
			name:   "weighted shares",
			shares: []share{NewShare(alice, 2), NewShare(bob, 1)},
			amount: eur(300),
			want:   map[string]int64{"alice": 200, "bob": 100},
		},
		{
			name:   "weighted shares with remainder",
			shares: []share{NewShare(alice, 1), NewShare(bob, 2)},
			amount: eur(100),
			want:   map[string]int64{"alice": 34, "bob": 66},
		},
		{
			name:   "single participant",
			shares: []share{NewShare(alice, 1)},
			amount: eur(999),
			want:   map[string]int64{"alice": 999},
		},
		{
			name:    "empty shares",
			shares:  []share{},
			amount:  eur(100),
			wantErr: true,
		},
		{
			name:    "zero shares",
			shares:  []share{NewShare(alice, 0), NewShare(bob, 0)},
			amount:  eur(100),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := NewShareLayout(tt.shares...)
			got, err := layout.Split(tt.amount)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertParts(t, got, tt.want)
		})
	}
}

func TestEvenLayout_Split(t *testing.T) {
	alice := testParticipant("alice")
	bob := testParticipant("bob")
	charlie := testParticipant("charlie")

	tests := []struct {
		name   string
		people []Participant
		amount *money.Money
		want   map[string]int64
	}{
		{
			name:   "two people even",
			people: []Participant{alice, bob},
			amount: eur(200),
			want:   map[string]int64{"alice": 100, "bob": 100},
		},
		{
			name:   "three people with remainder",
			people: []Participant{alice, bob, charlie},
			amount: eur(10),
			want:   map[string]int64{"alice": 4, "bob": 3, "charlie": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := NewEvenLayout(tt.people...)
			got, err := layout.Split(tt.amount)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertParts(t, got, tt.want)
		})
	}
}

func TestPctLayout_Split(t *testing.T) {
	alice := testParticipant("alice")
	bob := testParticipant("bob")
	charlie := testParticipant("charlie")

	mustPct := func(p Participant, ppct uint) pct {
		v, err := NewPct(p, ppct)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	tests := []struct {
		name    string
		pcts    []pct
		amount  *money.Money
		want    map[string]int64
		wantErr bool
	}{
		{
			name:   "50/50 split",
			pcts:   []pct{mustPct(alice, 5000), mustPct(bob, 5000)},
			amount: eur(1000),
			want:   map[string]int64{"alice": 500, "bob": 500},
		},
		{
			name:   "70/30 split",
			pcts:   []pct{mustPct(alice, 7000), mustPct(bob, 3000)},
			amount: eur(1000),
			want:   map[string]int64{"alice": 700, "bob": 300},
		},
		{
			name:   "rounding goes to first participant",
			pcts:   []pct{mustPct(alice, 3333), mustPct(bob, 3333), mustPct(charlie, 3334)},
			amount: eur(100),
			want:   map[string]int64{"alice": 34, "bob": 33, "charlie": 33},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout, err := NewPctLayout(tt.pcts...)
			if err != nil {
				t.Fatalf("unexpected layout error: %v", err)
			}
			got, err := layout.Split(tt.amount)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertParts(t, got, tt.want)
		})
	}
}

func TestNewPctLayout_Validation(t *testing.T) {
	alice := testParticipant("alice")
	bob := testParticipant("bob")

	mustPct := func(p Participant, ppct uint) pct {
		v, err := NewPct(p, ppct)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	tests := []struct {
		name    string
		pcts    []pct
		wantErr string
	}{
		{
			name:    "less than 100%",
			pcts:    []pct{mustPct(alice, 5000), mustPct(bob, 2000)},
			wantErr: "less than 100%",
		},
		{
			name:    "more than 100%",
			pcts:    []pct{mustPct(alice, 6000), mustPct(bob, 6000)},
			wantErr: "more than 100%",
		},
		{
			name: "valid exact 100%",
			pcts: []pct{mustPct(alice, 5000), mustPct(bob, 5000)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPctLayout(tt.pcts...)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNewPct_Validation(t *testing.T) {
	alice := testParticipant("alice")

	_, err := NewPct(alice, HundredPercent+1)
	if err == nil {
		t.Fatal("expected error for ppct > 100%")
	}

	_, err = NewPct(alice, 0)
	if err == nil {
		t.Fatal("expected error for ppct == 0")
	}
}

// fixedRateConverter converts at a fixed 1:1 rate (just changes currency code).
type fixedRateConverter struct{}

func (fixedRateConverter) Convert(a *money.Money, target money.Currency) (*money.Money, error) {
	return money.New(a.Amount(), target.Code), nil
}

func TestAbsLayout_Split(t *testing.T) {
	alice := testParticipant("alice")
	bob := testParticipant("bob")
	charlie := testParticipant("charlie")

	conv := fixedRateConverter{}

	tests := []struct {
		name    string
		abs     []ParticipantExpence
		rest    ExpenseLayout
		amount  *money.Money
		want    []expectedExpence
		wantErr bool
	}{
		{
			name: "absolute only, no rest",
			abs: []ParticipantExpence{
				NewParticipantExpence(alice, eur(60)),
				NewParticipantExpence(bob, eur(40)),
			},
			rest:   nil,
			amount: eur(100),
			want: []expectedExpence{
				{"alice", 60, "EUR"},
				{"bob", 40, "EUR"},
			},
		},
		{
			name: "absolute with even rest",
			abs: []ParticipantExpence{
				NewParticipantExpence(alice, eur(50)),
			},
			rest:   NewEvenLayout(bob, charlie),
			amount: eur(100),
			want: []expectedExpence{
				{"alice", 50, "EUR"},
				{"bob", 25, "EUR"},
				{"charlie", 25, "EUR"},
			},
		},
		{
			name: "absolute with share rest",
			abs: []ParticipantExpence{
				NewParticipantExpence(alice, eur(100)),
			},
			rest:   NewShareLayout(NewShare(bob, 2), NewShare(charlie, 1)),
			amount: eur(400),
			want: []expectedExpence{
				{"alice", 100, "EUR"},
				{"bob", 200, "EUR"},
				{"charlie", 100, "EUR"},
			},
		},
		{
			name: "absolute exceeds total",
			abs: []ParticipantExpence{
				NewParticipantExpence(alice, eur(200)),
			},
			rest:    NewEvenLayout(bob),
			amount:  eur(100),
			wantErr: true,
		},
		{
			name: "absolute equals total, no rest layout",
			abs: []ParticipantExpence{
				NewParticipantExpence(alice, eur(100)),
			},
			rest:   nil,
			amount: eur(100),
			want: []expectedExpence{
				{"alice", 100, "EUR"},
			},
		},
		{
			name: "cross-currency absolute reduces remainder via conversion",
			abs: []ParticipantExpence{
				NewParticipantExpence(alice, money.New(10, "USD")),
			},
			rest:   NewEvenLayout(alice, bob, charlie),
			amount: eur(100),
			// 10 USD converts to 10 EUR (1:1), remainder = 90 EUR / 3 = 30 each
			want: []expectedExpence{
				{"alice", 10, "USD"},
				{"alice", 30, "EUR"},
				{"bob", 30, "EUR"},
				{"charlie", 30, "EUR"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := NewAbsLayout(conv, tt.rest, tt.abs...)
			got, err := layout.Split(tt.amount)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertPartsOrdered(t, got, tt.want)
		})
	}
}

type expectedExpence struct {
	id       string
	amount   int64
	currency string
}

func assertPartsOrdered(t *testing.T, got []ParticipantExpence, want []expectedExpence) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d parts, want %d", len(got), len(want))
	}

	for i, w := range want {
		g := got[i]
		id := g.participant.ParticipantID()
		if id != w.id {
			t.Errorf("[%d] participant: got %q, want %q", i, id, w.id)
		}
		if g.amount.Amount() != w.amount {
			t.Errorf("[%d] %s amount: got %d, want %d", i, id, g.amount.Amount(), w.amount)
		}
		if g.amount.Currency().Code != w.currency {
			t.Errorf("[%d] %s currency: got %q, want %q", i, id, g.amount.Currency().Code, w.currency)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
