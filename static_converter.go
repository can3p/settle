package settle

import (
	"errors"

	"github.com/Rhymond/go-money"
	"github.com/shopspring/decimal"
)

var ErrRateNotFound = errors.New("rate not found")

var _ CurrencyConverter = (*staticConverter)(nil)

type staticConverter struct {
	conversionMap map[string]decimal.Decimal
}

func NewStaticConverter() *staticConverter {
	return &staticConverter{
		conversionMap: make(map[string]decimal.Decimal),
	}
}

// AddRate registers a conversion rate between two currencies.
// The rate is expressed in full currency units, e.g. for USD→JPY
// with a market rate of 150 yen per dollar, pass rate = 150.
func (s *staticConverter) AddRate(from string, to string, rate decimal.Decimal) {
	s.conversionMap[from+to] = rate
	s.conversionMap[to+from] = decimal.NewFromInt(1).Div(rate)
}

// Convert converts a monetary amount to the target currency using the
// registered rate. It accounts for differing fractional digits between
// currencies (e.g. USD stores cents with Fraction=2, JPY stores whole
// yen with Fraction=0).
func (s *staticConverter) Convert(a *money.Money, target money.Currency) (*money.Money, error) {
	rate, ok := s.conversionMap[a.Currency().Code+target.Code]
	if !ok {
		return nil, ErrRateNotFound
	}

	sourceAmount := decimal.NewFromInt(a.Amount())

	// Shift adjusts for the difference in fractional digits.
	// Example: USD (Fraction=2) → JPY (Fraction=0) means Shift(-2),
	// i.e. divide by 100, because 10000 cents * 150 / 100 = 15000 yen.
	shift := int32(target.Fraction) - int32(a.Currency().Fraction)
	result := sourceAmount.Mul(rate).Shift(shift)

	return money.New(result.Round(0).IntPart(), target.Code), nil
}
