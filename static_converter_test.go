package settle

import (
	"errors"
	"testing"

	"github.com/Rhymond/go-money"
	"github.com/shopspring/decimal"
)

func usd(amount int64) *money.Money { return money.New(amount, money.USD) }
func jpy(amount int64) *money.Money { return money.New(amount, money.JPY) }

func jpyCurrency() money.Currency { return *money.GetCurrency(money.JPY) }
func usdCurrency() money.Currency { return *money.GetCurrency(money.USD) }

func TestStaticConverter_Convert_SameFraction(t *testing.T) {
	// EUR and USD both have Fraction=2 (cents)
	conv := NewStaticConverter()
	conv.AddRate(money.EUR, money.USD, decimal.NewFromFloat(1.1))

	tests := []struct {
		name   string
		input  *money.Money
		target money.Currency
		want   int64
	}{
		{
			name:   "100 EUR to USD at 1.1 = 110 USD",
			input:  eur(100_00),
			target: usdCurrency(),
			want:   110_00,
		},
		{
			name:   "1 EUR to USD at 1.1 = 1.10 USD",
			input:  eur(1_00),
			target: usdCurrency(),
			want:   1_10,
		},
		{
			name:   "50 EUR to USD at 1.1 = 55 USD",
			input:  eur(50_00),
			target: usdCurrency(),
			want:   55_00,
		},
		{
			name:   "0.10 EUR to USD at 1.1 = 0.11 USD",
			input:  eur(10),
			target: usdCurrency(),
			want:   11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := conv.Convert(tt.input, tt.target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Amount() != tt.want {
				t.Errorf("got %d, want %d", got.Amount(), tt.want)
			}
			if got.Currency().Code != tt.target.Code {
				t.Errorf("currency: got %s, want %s", got.Currency().Code, tt.target.Code)
			}
		})
	}
}

func TestStaticConverter_Convert_USDtoJPY(t *testing.T) {
	// USD has Fraction=2 (cents), JPY has Fraction=0 (whole yen)
	// Market rate: 1 USD = 150 JPY
	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.JPY, decimal.NewFromInt(150))

	tests := []struct {
		name   string
		input  *money.Money
		target money.Currency
		want   int64
	}{
		{
			name:   "100 USD to JPY (10000 cents -> 15000 yen)",
			input:  usd(100_00),
			target: jpyCurrency(),
			want:   15000,
		},
		{
			name:   "1 USD to JPY (100 cents -> 150 yen)",
			input:  usd(1_00),
			target: jpyCurrency(),
			want:   150,
		},
		{
			name:   "0.50 USD to JPY (50 cents -> 75 yen)",
			input:  usd(50),
			target: jpyCurrency(),
			want:   75,
		},
		{
			name:   "10 USD to JPY",
			input:  usd(10_00),
			target: jpyCurrency(),
			want:   1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := conv.Convert(tt.input, tt.target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Amount() != tt.want {
				t.Errorf("got %d, want %d", got.Amount(), tt.want)
			}
			if got.Currency().Code != tt.target.Code {
				t.Errorf("currency: got %s, want %s", got.Currency().Code, tt.target.Code)
			}
		})
	}
}

func TestStaticConverter_Convert_JPYtoUSD(t *testing.T) {
	// Reverse direction: JPY (Fraction=0) -> USD (Fraction=2)
	// Market rate: 1 USD = 150 JPY, so 1 JPY = 1/150 USD
	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.JPY, decimal.NewFromInt(150))

	tests := []struct {
		name   string
		input  *money.Money
		target money.Currency
		want   int64
	}{
		{
			name:   "15000 JPY to USD (-> 10000 cents = $100)",
			input:  jpy(15000),
			target: usdCurrency(),
			want:   100_00,
		},
		{
			name:   "150 JPY to USD (-> 100 cents = $1)",
			input:  jpy(150),
			target: usdCurrency(),
			want:   1_00,
		},
		{
			name:   "75 JPY to USD (-> 50 cents)",
			input:  jpy(75),
			target: usdCurrency(),
			want:   50,
		},
		{
			name:   "100 JPY to USD (-> 66.67 cents, rounds to 67)",
			input:  jpy(100),
			target: usdCurrency(),
			want:   67,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := conv.Convert(tt.input, tt.target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Amount() != tt.want {
				t.Errorf("got %d, want %d", got.Amount(), tt.want)
			}
			if got.Currency().Code != tt.target.Code {
				t.Errorf("currency: got %s, want %s", got.Currency().Code, tt.target.Code)
			}
		})
	}
}

func TestStaticConverter_Convert_InverseRateAutoRegistered(t *testing.T) {
	// Adding USD->EUR should automatically register EUR->USD
	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.EUR, decimal.NewFromFloat(0.9))

	// Forward: 100 USD = 90 EUR
	got, err := conv.Convert(usd(100_00), eurCurrency())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount() != 90_00 {
		t.Errorf("forward: got %d, want %d", got.Amount(), int64(90_00))
	}

	// Reverse: 90 EUR = 100 USD
	got, err = conv.Convert(eur(90_00), usdCurrency())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount() != 100_00 {
		t.Errorf("reverse: got %d, want %d", got.Amount(), int64(100_00))
	}
}

func TestStaticConverter_Convert_RateNotFound(t *testing.T) {
	conv := NewStaticConverter()

	_, err := conv.Convert(usd(100_00), eurCurrency())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRateNotFound) {
		t.Errorf("got error %v, want ErrRateNotFound", err)
	}
}

func TestStaticConverter_Convert_ZeroAmount(t *testing.T) {
	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.EUR, decimal.NewFromFloat(0.9))

	got, err := conv.Convert(usd(0), eurCurrency())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount() != 0 {
		t.Errorf("got %d, want 0", got.Amount())
	}
}

func TestStaticConverter_Convert_LargeAmount(t *testing.T) {
	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.JPY, decimal.NewFromInt(150))

	// $1,000,000 = 100_000_000 cents -> 150_000_000 yen
	got, err := conv.Convert(usd(100_000_000), jpyCurrency())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount() != 150_000_000 {
		t.Errorf("got %d, want %d", got.Amount(), int64(150_000_000))
	}
}

func TestStaticConverter_Convert_FractionalRate(t *testing.T) {
	// Rate with many decimals: 1 USD = 149.53 JPY
	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.JPY, decimal.NewFromFloat(149.53))

	// $100 = 10000 cents -> 10000 * 149.53 / 100 = 14953 yen
	got, err := conv.Convert(usd(100_00), jpyCurrency())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount() != 14953 {
		t.Errorf("got %d, want %d", got.Amount(), int64(14953))
	}
}

func TestStaticConverter_Convert_RoundingBehavior(t *testing.T) {
	conv := NewStaticConverter()
	// 1 EUR = 1.15 USD — 1 EUR cent becomes 1.15 USD cents -> rounds to 1
	conv.AddRate(money.EUR, money.USD, decimal.NewFromFloat(1.15))

	tests := []struct {
		name  string
		input *money.Money
		want  int64
	}{
		{
			name:  "1 cent rounds down",
			input: eur(1),
			want:  1, // 1 * 1.15 = 1.15 -> rounds to 1 (half-up rounding at .5)
		},
		{
			name:  "3 cents",
			input: eur(3),
			want:  3, // 3 * 1.15 = 3.45 -> rounds to 3
		},
		{
			name:  "10 cents",
			input: eur(10),
			want:  12, // 10 * 1.15 = 11.5 -> rounds to 12
		},
		{
			name:  "100 cents",
			input: eur(100),
			want:  115,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := conv.Convert(tt.input, usdCurrency())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Amount() != tt.want {
				t.Errorf("got %d, want %d", got.Amount(), tt.want)
			}
		})
	}
}

func TestStaticConverter_AddRate_OverwritesPrevious(t *testing.T) {
	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.EUR, decimal.NewFromFloat(0.9))
	conv.AddRate(money.USD, money.EUR, decimal.NewFromFloat(0.8))

	got, err := conv.Convert(usd(100_00), eurCurrency())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Amount() != 80_00 {
		t.Errorf("got %d, want %d (should use latest rate)", got.Amount(), int64(80_00))
	}
}

func TestStaticConverter_MultipleRates(t *testing.T) {
	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.EUR, decimal.NewFromFloat(0.9))
	conv.AddRate(money.USD, money.JPY, decimal.NewFromInt(150))
	conv.AddRate(money.EUR, money.JPY, decimal.NewFromInt(160))

	tests := []struct {
		name   string
		input  *money.Money
		target money.Currency
		want   int64
	}{
		{"USD to EUR", usd(100_00), eurCurrency(), 90_00},
		{"USD to JPY", usd(100_00), jpyCurrency(), 15000},
		{"EUR to JPY", eur(100_00), jpyCurrency(), 16000},
		{"EUR to USD (auto inverse)", eur(90_00), usdCurrency(), 100_00},
		{"JPY to USD (auto inverse)", jpy(15000), usdCurrency(), 100_00},
		{"JPY to EUR (auto inverse)", jpy(16000), eurCurrency(), 100_00},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := conv.Convert(tt.input, tt.target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Amount() != tt.want {
				t.Errorf("got %d, want %d", got.Amount(), tt.want)
			}
		})
	}
}

func TestStaticConverter_SettlementIntegration(t *testing.T) {
	// Full integration: settlement with multi-currency expenses
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")

	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.JPY, decimal.NewFromInt(150))

	// Settlement in JPY. Alice pays $20 (= 2000 cents) split evenly with Bob.
	// After conversion: 2000 * 150 / 100 = 3000 JPY total, Bob owes 1500 JPY.
	s := NewSettlement(jpyCurrency(), conv,
		NewExpense(alice, usd(20_00), NewEvenLayout(alice, bob)),
	)

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertSettlement(t, result, map[string]map[string]int64{
		"bob": {"alice": 1500},
	})
}

func TestStaticConverter_SettlementMixedCurrencies(t *testing.T) {
	alice := StringParticipant("alice")
	bob := StringParticipant("bob")
	charlie := StringParticipant("charlie")

	conv := NewStaticConverter()
	conv.AddRate(money.USD, money.EUR, decimal.NewFromFloat(0.9))
	conv.AddRate(money.JPY, money.EUR, decimal.NewFromFloat(0.006))

	// Settlement in EUR. Mixed currency expenses.
	s := NewSettlement(eurCurrency(), conv,
		// Alice pays $100 (10000 cents) for everyone = 9000 EUR cents = 90 EUR
		NewExpense(alice, usd(100_00), NewEvenLayout(alice, bob, charlie)),
		// Bob pays 10000 JPY for everyone = 10000 * 0.006 = 60, shift(+2) = 60 * 100 = 6000 EUR cents = 60 EUR
		NewExpense(bob, jpy(10000), NewEvenLayout(alice, bob, charlie)),
	)

	result, err := s.Settle()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Alice's expense: 90 EUR split 3 ways = 30 EUR each
	//   bob -> alice: 30_00, charlie -> alice: 30_00
	// Bob's expense: 60 EUR split 3 ways = 20 EUR each
	//   alice -> bob: 20_00, charlie -> bob: 20_00
	// Net: bob -> alice: 30_00 - 20_00 = 10_00
	//      charlie -> alice: 30_00
	//      charlie -> bob: 20_00
	assertSettlement(t, result, map[string]map[string]int64{
		"bob":     {"alice": 10_00},
		"charlie": {"alice": 30_00, "bob": 20_00},
	})
}
