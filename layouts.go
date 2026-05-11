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

func (s shareLayout) Split(a money.Money) ([]ParticipantExpence, error) {
	return nil, errors.ErrUnsupported
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
	ppct uint // 0.25% = 25 ppct, 0 = the remainder
}

const HundredPercent = 10_000

func NewPct(p Participant, ppct uint) (pct, error) {
	if ppct > HundredPercent {
		return pct{}, errors.New("a percent share cannot be more than 100%")
	}

	return pct{p, ppct}, nil
}

type pctLayout struct {
	shares []pct
}

func (s pctLayout) Split(a money.Money) ([]ParticipantExpence, error) {
	return nil, errors.ErrUnsupported
}

func NewPctLayout(pct ...pct) (pctLayout, error) {
	total := uint(0)

	for _, p := range pct {
		total += p.ppct
	}

	if total < HundredPercent {
		return pctLayout{}, errors.New("all percentages add up to less than 100%")
	}

	if total < HundredPercent {
		return pctLayout{}, errors.New("all percentages add up to more than 100%")
	}

	return pctLayout{pct}, nil
}

type absLayout struct {
	expenses []ParticipantExpence
	theRest  ExpenseLayout
}

func NewAbsLayout(rest ExpenseLayout, abs ...ParticipantExpence) absLayout {
	return absLayout{abs, rest}
}

func (s absLayout) Split(a money.Money) ([]ParticipantExpence, error) {
	return nil, errors.ErrUnsupported
}
