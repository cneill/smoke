package rank

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
)

type Item struct {
	ID          string `json:"id"`
	Contents    string `json:"contents"`
	RankHistory []int  `json:"-"`
}

func (i *Item) Clone() *Item {
	clonedHistory := make([]int, len(i.RankHistory))
	copy(clonedHistory, i.RankHistory)

	return &Item{
		ID:          i.ID,
		Contents:    i.Contents,
		RankHistory: clonedHistory,
	}
}

func (i *Item) RankingScore() float64 {
	var sum float64
	for _, rank := range i.RankHistory {
		sum += float64(rank)
	}

	// Score = the average ranking across history.
	return sum / float64(len(i.RankHistory))
}

type Items []*Item

func (i Items) Shuffle() {
	rand.Shuffle(len(i), func(a, b int) {
		i[a], i[b] = i[b], i[a]
	})
}

func (i Items) Batch(requestedSize int) ([]Items, error) {
	num := len(i)
	if num < 2 {
		return nil, fmt.Errorf("must have at least 2 items for batching, have %d", num)
	}

	// We must have AT LEAST 2 items per batch.
	requestedSize = max(2, requestedSize)
	requestedBatches := int(math.Ceil(float64(num) / float64(requestedSize)))

	// We will end up with AT LEAST 1 batch and AT MOST num/2 batches to ensure we have at least 2 items per batch.
	maxBatches := max(num/2, 1)
	numBatches := max(1, requestedBatches)
	numBatches = min(maxBatches, numBatches)

	cloned := i.Clone()
	cloned.Shuffle()

	base := num / numBatches
	remainder := num % numBatches

	results := make([]Items, numBatches)
	start := 0

	for batchIdx := range numBatches {
		batchSize := base
		if batchIdx < remainder {
			batchSize++
		}

		end := start + batchSize

		batch := make(Items, batchSize)
		copy(batch, cloned[start:end])

		results[batchIdx] = batch
		start = end
	}

	return results, nil
}

func (i Items) Clone() Items {
	cloned := make(Items, len(i))

	for idx, item := range i {
		cloned[idx] = item.Clone()
	}

	return cloned
}

func (i Items) JSON() string {
	bytes, _ := json.Marshal(i)
	return string(bytes)
}

func (i Items) AddRankings(ids []string) error {
	if len(ids) != len(i) {
		return fmt.Errorf("missing IDs; expected %d, got %d", len(i), len(ids))
	}

	clonedIDs := make([]string, len(ids))
	copy(clonedIDs, ids)
	slices.Sort(clonedIDs)

	compacted := slices.Compact(clonedIDs)
	if len(compacted) != len(ids) {
		return fmt.Errorf("got duplicated IDs in rankings: %s", strings.Join(clonedIDs, ", "))
	}

	for rank, id := range ids {
		foundItem := false

		for _, item := range i {
			if item.ID == id {
				item.RankHistory = append(item.RankHistory, rank)
				foundItem = true

				break
			}
		}

		if !foundItem {
			return fmt.Errorf("got an unrecognized ID: %q", id)
		}
	}

	return nil
}

func (i Items) RankSorted() Items {
	sorted := i.Clone()
	slices.SortFunc(sorted, func(a, b *Item) int {
		return cmp.Compare(a.RankingScore(), b.RankingScore())
	})

	return sorted
}

func MergeBatches(batches ...Items) Items {
	result := Items{}
	idMap := map[string]Items{}

	for _, batchItems := range batches {
		for _, item := range batchItems {
			idMap[item.ID] = append(idMap[item.ID], item)
		}
	}

	for id, items := range idMap {
		newItem := &Item{
			ID: id,
		}

		history := []int{}
		for _, item := range items {
			history = append(history, item.RankHistory...)
			newItem.Contents = item.Contents
		}

		newItem.RankHistory = history

		result = append(result, newItem)
	}

	return result.Clone()
}
