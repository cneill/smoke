package rank

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
)

type Item struct {
	ID       string `json:"id"`
	Contents string `json:"contents"`
	History  []int  `json:"-"`
}

func (i *Item) Clone() *Item {
	temp := *i
	return &temp
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
