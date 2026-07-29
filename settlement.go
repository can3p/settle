package settle

import (
	"slices"
	"strings"

	"github.com/Rhymond/go-money"
)

type Settlement struct {
	currency   money.Currency
	converter  CurrencyConverter
	expenses   []Expense
	repayments []Transfer
	optimizer  Optimizer
}

func NewSettlement(c money.Currency, converter CurrencyConverter, e ...Expense) Settlement {
	return Settlement{c, converter, e, nil, nil}
}

func (s Settlement) AddExpenses(expenses ...Expense) Settlement {
	return Settlement{s.currency, s.converter, append(s.expenses, expenses...), s.repayments, s.optimizer}
}

func (s Settlement) AddRepayments(repayments ...Transfer) Settlement {
	return Settlement{s.currency, s.converter, s.expenses, append(s.repayments, repayments...), s.optimizer}
}

func (s Settlement) Settle() (SettlementResult, error) {
	sMap := map[string]map[string]Transfer{} // sMap[from][to]transfer

	hasTransfer := func(from, to string) (Transfer, bool) {
		sFrom, ok := sMap[from]

		if !ok {
			return Transfer{}, false
		}

		sTo, ok := sFrom[to]

		return sTo, ok
	}

	totalTransfers := []Transfer{}

	// repayments passed to the settlement highlight that the recipient already paid the sender
	// so we need to reverse the transfer
	for _, t := range s.repayments {
		totalTransfers = append(totalTransfers, Transfer{From: t.To, To: t.From, Amount: t.Amount})
	}

	for _, e := range s.expenses {
		transfers, err := e.Split()

		if err != nil {
			return nil, err
		}
		totalTransfers = append(totalTransfers, transfers...)
	}

	for _, t := range totalTransfers {
		// we could have made a conversion to the target currency
		// right at expense definition, but we didn't to allow
		// to specify individual absolute expenses in different
		// currencies
		t, err := t.Convert(s.currency, s.converter)
		if err != nil {
			return nil, err
		}

		from := t.From.ParticipantID()
		to := t.To.ParticipantID()

		if existing, ok := hasTransfer(from, to); ok {
			added, err := existing.Add(t.Amount)

			if err != nil {
				return nil, err
			}

			sMap[from][to] = added
		} else if reverse, ok := hasTransfer(to, from); ok {
			equals, err := reverse.Amount.Equals(t.Amount)
			if err != nil {
				return nil, err
			}

			if equals {
				delete(sMap[to], from) // transfers in opposite directions annihilate each other
				continue
			}

			bigger, err := reverse.Amount.GreaterThan(t.Amount)
			if err != nil {
				return nil, err
			}

			if bigger {
				diff, err := reverse.Amount.Subtract(t.Amount)
				if err != nil {
					return nil, err
				}

				sMap[to][from] = Transfer{
					From:   reverse.From,
					To:     reverse.To,
					Amount: diff,
				}
				continue
			}

			diff, err := t.Amount.Subtract(reverse.Amount)
			if err != nil {
				return nil, err
			}

			if sMap[from] == nil {
				sMap[from] = map[string]Transfer{}
			}
			sMap[from][to] = Transfer{
				From:   t.From,
				To:     t.To,
				Amount: diff,
			}
			delete(sMap[to], from) // new transfer is bigger than reverse transfer
		} else {
			if sMap[from] == nil {
				sMap[from] = map[string]Transfer{}
			}
			sMap[from][to] = t
		}
	}

	result := SettlementResult{}

	for _, from := range sMap {
		for _, t := range from {
			result = append(result, t)
		}
	}

	if s.optimizer != nil {
		optimized, err := s.optimizer(result)
		if err != nil {
			return nil, err
		}
		result = optimized
	}

	slices.SortFunc(result, func(a, b Transfer) int {
		cmpFrom := strings.Compare(a.From.ParticipantID(), b.From.ParticipantID())

		if cmpFrom != 0 {
			return cmpFrom
		}

		return strings.Compare(a.To.ParticipantID(), b.To.ParticipantID())
	})

	return result, nil
}

func (e Expense) Split() (SettlementResult, error) {
	r := SettlementResult{}

	parts, err := e.layout.Split(e.amount)

	if err != nil {
		return nil, err
	}

	for _, p := range parts {
		if p.participant.ParticipantID() == e.payer.ParticipantID() {
			continue
		}

		r = append(r, Transfer{
			From:   p.participant,
			To:     e.payer,
			Amount: p.amount,
		})
	}

	return r, nil
}
