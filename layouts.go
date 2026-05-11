package settle

import (
	"errors"

	"github.com/Rhymond/go-money"
)

type share struct {
	p   Participant
	num uint
}

func NewShare(p Participant, numShares uint) share {
	return share{p, numShares}
}

type shareLayout struct {
	shares []share
}

func (s shareLayout) Split(a *money.Money) ([]ParticipantExpence, error) {
	if len(s.shares) == 0 {
		return nil, errors.New("no participants in layout")
	}

	ratios := make([]int, len(s.shares))
	total := 0
	for i, sh := range s.shares {
		ratios[i] = int(sh.num)
		total += ratios[i]
	}

	if total == 0 {
		return nil, errors.New("total shares cannot be zero")
	}

	parts, err := a.Allocate(ratios...)
	if err != nil {
		return nil, err
	}

	result := make([]ParticipantExpence, len(s.shares))
	for i, sh := range s.shares {
		result[i] = ParticipantExpence{
			participant: sh.p,
			amount:      parts[i],
		}
	}

	return result, nil
}

func NewShareLayout(s ...share) shareLayout {
	return shareLayout{s}
}

func NewEvenLayout(p ...Participant) shareLayout {
	s := make([]share, 0, len(p))

	for _, p := range p {
		s = append(s, NewShare(p, 1))
	}

	return NewShareLayout(s...)
}

type pct struct {
	p    Participant
	ppct uint // 0.25% = 25 ppct
}

const HundredPercent = 10_000

func NewPct(p Participant, ppct uint) (pct, error) {
	if ppct == 0 {
		return pct{}, errors.New("a percent share cannot be zero")
	}

	if ppct > HundredPercent {
		return pct{}, errors.New("a percent share cannot be more than 100%")
	}

	return pct{p, ppct}, nil
}

type pctLayout struct {
	shares []pct
}

func (s pctLayout) Split(a *money.Money) ([]ParticipantExpence, error) {
	if len(s.shares) == 0 {
		return nil, errors.New("no participants in layout")
	}

	ratios := make([]int, len(s.shares))
	for i, p := range s.shares {
		ratios[i] = int(p.ppct)
	}

	parts, err := a.Allocate(ratios...)
	if err != nil {
		return nil, err
	}

	result := make([]ParticipantExpence, len(s.shares))
	for i, p := range s.shares {
		result[i] = ParticipantExpence{
			participant: p.p,
			amount:      parts[i],
		}
	}

	return result, nil
}

func NewPctLayout(pcts ...pct) (pctLayout, error) {
	total := uint(0)

	for _, p := range pcts {
		total += p.ppct
	}

	if total != HundredPercent {
		if total < HundredPercent {
			return pctLayout{}, errors.New("all percentages add up to less than 100%")
		}
		return pctLayout{}, errors.New("all percentages add up to more than 100%")
	}

	return pctLayout{pcts}, nil
}

type absLayout struct {
	expenses  []ParticipantExpence
	theRest   ExpenseLayout
	converter CurrencyConverter
}

func NewAbsLayout(converter CurrencyConverter, rest ExpenseLayout, abs ...ParticipantExpence) absLayout {
	return absLayout{abs, rest, converter}
}

func (s absLayout) Split(a *money.Money) ([]ParticipantExpence, error) {
	result := make([]ParticipantExpence, 0, len(s.expenses))
	remaining := a.Amount()
	targetCode := a.Currency().Code

	for _, e := range s.expenses {
		result = append(result, e)

		if e.amount.Currency().Code == targetCode {
			remaining -= e.amount.Amount()
		} else {
			// Convert cross-currency absolute to the expense currency
			// to determine how much it reduces the remaining pool.
			converted, err := s.converter.Convert(e.amount, *a.Currency())
			if err != nil {
				return nil, err
			}
			remaining -= converted.Amount()
		}
	}

	if remaining < 0 {
		return nil, errors.New("absolute amounts exceed the total")
	}

	if s.theRest != nil && remaining > 0 {
		restAmount := money.New(remaining, targetCode)
		restParts, err := s.theRest.Split(restAmount)
		if err != nil {
			return nil, err
		}
		result = append(result, restParts...)
	}

	return result, nil
}
