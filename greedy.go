package settle

import (
	"slices"
	"strings"

	"github.com/Rhymond/go-money"
)

// participantBalance tracks a participant's net position after all transfers
// have been netted out.
//
// A positive balance means the participant is owed money (creditor).
// A negative balance means the participant owes money (debtor).
type participantBalance struct {
	participant Participant
	amount      int64
}

// GreedyOptimizer reduces the number of transactions by greedily
// matching the participant who is owed the most with the participant who
// owes the most. This is the same approach described in articles examining
// the Splitwise algorithm, e.g.
// https://medium.com/@howoftech/how-does-the-splitwise-algorithm-work-dc1de5eaa371
//
// The algorithm works in two phases:
//  1. Compute the net balance for every participant from the raw
//     settlement result.
//  2. Sort participants by balance (debtors first, creditors last) and use
//     a two-pointer technique: the leftmost entry is the max debtor and the
//     rightmost is the max creditor. Settle the smaller of the two balances
//     between them, then advance the pointer of whichever participant was
//     zeroed out. Entries that cross to the wrong side (e.g. a debtor that
//     becomes a creditor) are skipped. This zeroes out at least one
//     participant per iteration, so the result has at most n-1 transfers
//     for n participants with non-zero balances.
//
// The net balance of each participant is preserved — only the number of
// transactions is reduced.
//
// This function can be passed to WithOptimizer:
//
//	s := settle.NewSettlement(eur, conv, expenses...).
//	    WithOptions(settle.WithOptimizer(settle.GreedyOptimizer))
func GreedyOptimizer(result SettlementResult) (SettlementResult, error) {
	if len(result) == 0 {
		return result, nil
	}

	// All transfers in the result are already in the settlement currency.
	currency := result[0].Amount.Currency()

	// Phase 1: compute net balances.
	balances := map[string]*participantBalance{}

	for _, t := range result {
		fromID := t.From.ParticipantID()
		toID := t.To.ParticipantID()

		bFrom, ok := balances[fromID]
		if !ok {
			bFrom = &participantBalance{participant: t.From}
			balances[fromID] = bFrom
		}
		bTo, ok := balances[toID]
		if !ok {
			bTo = &participantBalance{participant: t.To}
			balances[toID] = bTo
		}

		// "from" pays "to": from's balance decreases, to's increases.
		bFrom.amount -= t.Amount.Amount()
		bTo.amount += t.Amount.Amount()
	}

	// Collect participants with non-zero balances.
	entries := make([]*participantBalance, 0, len(balances))
	for _, b := range balances {
		if b.amount != 0 {
			entries = append(entries, b)
		}
	}

	// Sort by balance ascending (debtors on the left, creditors on the
	// right). Ties are broken by participant ID for deterministic output.
	slices.SortFunc(entries, func(a, b *participantBalance) int {
		if a.amount != b.amount {
			if a.amount < b.amount {
				return -1
			}
			return 1
		}
		return strings.Compare(a.participant.ParticipantID(), b.participant.ParticipantID())
	})

	// Phase 2: two-pointer greedy settlement.
	optimized := make(SettlementResult, 0, len(entries))

	left, right := 0, len(entries)-1

	for left < right {
		// Skip creditors that ended up on the left side (a debtor that
		// became a creditor after a previous settlement).
		for left < right && entries[left].amount >= 0 {
			left++
		}
		// Skip debtors that ended up on the right side (a creditor that
		// became a debtor after a previous settlement).
		for left < right && entries[right].amount <= 0 {
			right--
		}
		if left >= right {
			break
		}

		debtor := entries[left]
		creditor := entries[right]

		// Transfer the smaller of the two absolute balances.
		amount := -debtor.amount
		if creditor.amount < amount {
			amount = creditor.amount
		}

		optimized = append(optimized, Transfer{
			From:   debtor.participant,
			To:     creditor.participant,
			Amount: money.New(amount, currency.Code),
		})

		debtor.amount += amount
		creditor.amount -= amount

		// Advance the pointer of whichever participant was zeroed out.
		if debtor.amount == 0 {
			left++
		}
		if creditor.amount == 0 {
			right--
		}
	}

	return optimized, nil
}
