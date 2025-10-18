package rank_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/cneill/smoke/pkg/commands/handlers/rank"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randomItems(t *testing.T, num int) rank.Items {
	t.Helper()

	items := make(rank.Items, num)
	for idx := range num {
		items[idx] = &rank.Item{
			ID:       fmt.Sprintf("item-%d", idx),
			Contents: "test",
		}
	}

	return items
}

func TestItems_Batch(t *testing.T) { //nolint:funlen
	t.Parallel()

	tests := []struct {
		numItems     int
		maxBatchSize int

		errors               bool
		expectedBatches      int
		expectedMinBatchSize int
		expectedMaxBatchSize int
	}{
		{
			numItems:     0,
			maxBatchSize: 5,
			errors:       true,
		},
		{
			numItems:     1,
			maxBatchSize: 5,
			errors:       true,
		},
		{
			numItems:             2,
			maxBatchSize:         -1,
			errors:               false,
			expectedBatches:      1,
			expectedMinBatchSize: 2,
			expectedMaxBatchSize: 2,
		},
		{
			numItems:             15,
			maxBatchSize:         -1,
			errors:               false,
			expectedBatches:      7,
			expectedMinBatchSize: 2,
			expectedMaxBatchSize: 3,
		},
		{
			numItems:             10,
			maxBatchSize:         5,
			errors:               false,
			expectedBatches:      2,
			expectedMinBatchSize: 5,
			expectedMaxBatchSize: 5,
		},
		{
			numItems:             10,
			maxBatchSize:         3,
			errors:               false,
			expectedBatches:      4,
			expectedMinBatchSize: 2,
			expectedMaxBatchSize: 3,
		},
		{
			numItems:             2,
			maxBatchSize:         3,
			errors:               false,
			expectedBatches:      1,
			expectedMinBatchSize: 2,
			expectedMaxBatchSize: 2,
		},
		{
			numItems:             100,
			maxBatchSize:         7,
			errors:               false,
			expectedBatches:      15,
			expectedMinBatchSize: 6,
			expectedMaxBatchSize: 7,
		},
	}

	for _, test := range tests {
		name := fmt.Sprintf("%d_items_%d_max_size", test.numItems, test.maxBatchSize)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			items := randomItems(t, test.numItems)

			batches, err := items.Batch(test.maxBatchSize)

			if test.errors {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			assert.Len(t, batches, test.expectedBatches)

			minBatchSize := math.MaxInt
			maxBatchSize := 0
			totalBatchItems := 0

			for _, batch := range batches {
				n := len(batch)
				assert.GreaterOrEqual(t, n, 2, "expecting every batch to have >=2 items, got %d", n)
				minBatchSize = min(minBatchSize, n)
				maxBatchSize = max(maxBatchSize, n)
				totalBatchItems += n

				// fmt.Printf("Batch index %d:\n", batchIdx)
				//
				// for itemIdx, item := range batch {
				// 	fmt.Printf("\t%d: %+v\n", itemIdx, item)
				// }
				//
				// fmt.Println()
			}

			assert.Equal(t, test.expectedMaxBatchSize, maxBatchSize, "expected max batch size %d, got %d", test.expectedMaxBatchSize, maxBatchSize)
			assert.Equal(t, test.expectedMinBatchSize, minBatchSize, "expected min batch size %d, got %d", test.expectedMinBatchSize, minBatchSize)
			assert.Equal(t, test.numItems, totalBatchItems, "missing items from initial Items slice")
			assert.GreaterOrEqual(t, 1, maxBatchSize-minBatchSize, "max batch size (%d) shouldn't differ from min batch (%d) size by more than 1", maxBatchSize, minBatchSize)
		})
	}
}
