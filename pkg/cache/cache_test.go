package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPropCache(t *testing.T) {
	// Arrange
	tests := map[string]struct {
		givenEntries  map[string]CacheEntry
		expectedCount int
	}{
		"cache has one entry": {
			givenEntries:  map[string]CacheEntry{"test1": {UpdatedAtMillis: 1, Props: make(map[string]string)}},
			expectedCount: 1,
		},
		"two entries for different repos": {
			givenEntries: map[string]CacheEntry{
				"test1": {UpdatedAtMillis: 1, Props: make(map[string]string)},
				"test2": {UpdatedAtMillis: 2, Props: make(map[string]string)},
			},
			expectedCount: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cache := NewPropCache()

			// Act
			cache.InsertMap(test.givenEntries)

			// Assert
			assert.Equal(t, test.expectedCount, len(cache.entries), fmt.Sprintf("the set should contain the correct number of entries: %+v", cache))
		})
	}
}

func testingEntryRefreshFn(ctx context.Context, name string, entry *CacheEntry) error {
	if name == "test-error" {
		return errors.New("failed")
	}

	entry.UpdatedAtMillis = time.Now().UnixMilli()

	return nil
}

func TestRefreshCacheAt(t *testing.T) {
	// Arrange
	tests := map[string]struct {
		givenCache           PropCache
		givenTimestamp       time.Time
		expectedSuccessCount int
		expectedErrs         error
	}{
		"should not refresh when no entries have expired": {
			givenCache: PropCache{
				expireAfter: 1 * time.Millisecond,
				entries: map[string]CacheEntry{
					"test1": {UpdatedAtMillis: 5, Props: make(map[string]string)},
					"test2": {UpdatedAtMillis: 10, Props: make(map[string]string)},
				},
				entryRefreshFn: testingEntryRefreshFn,
			},
			givenTimestamp:       time.UnixMilli(5),
			expectedSuccessCount: 0,
			expectedErrs:         nil,
		},
		"should refresh 1 entry when 1 entry has expired": {
			givenCache: PropCache{
				expireAfter: 1 * time.Millisecond,
				entries: map[string]CacheEntry{
					"test1": {UpdatedAtMillis: 5, Props: make(map[string]string)},
					"test2": {UpdatedAtMillis: 10, Props: make(map[string]string)},
				},
				entryRefreshFn: testingEntryRefreshFn,
			},
			givenTimestamp:       time.UnixMilli(6),
			expectedSuccessCount: 1,
			expectedErrs:         nil,
		},
		"should refresh 2 entries when 2 entries have expired": {
			givenCache: PropCache{
				expireAfter: 1 * time.Millisecond,
				entries: map[string]CacheEntry{
					"test1": {UpdatedAtMillis: 5, Props: make(map[string]string)},
					"test2": {UpdatedAtMillis: 10, Props: make(map[string]string)},
					"test3": {UpdatedAtMillis: 15, Props: make(map[string]string)},
				},
				entryRefreshFn: testingEntryRefreshFn,
			},
			givenTimestamp:       time.UnixMilli(11),
			expectedSuccessCount: 2,
			expectedErrs:         nil,
		},
		"should fail refresh gracefully and return any errors": {
			givenCache: PropCache{
				expireAfter: 1 * time.Millisecond,
				entries: map[string]CacheEntry{
					"test1":      {UpdatedAtMillis: 5, Props: make(map[string]string)},
					"test-error": {UpdatedAtMillis: 10, Props: make(map[string]string)},
				},
				entryRefreshFn: testingEntryRefreshFn,
			},
			givenTimestamp:       time.UnixMilli(11),
			expectedSuccessCount: 1,
			expectedErrs:         errors.Join(errors.New("failed")),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			successCount, errs := test.givenCache.RefreshCacheExpiredAt(context.Background(), test.givenTimestamp)

			// Assert
			assert.Equal(t, test.expectedErrs, errs)
			assert.Equal(t, test.expectedSuccessCount, successCount, fmt.Sprintf("the number of refreshed entries didn't match expected: %+v", test.givenCache))
		})
	}
}

func TestRefreshCacheForce(t *testing.T) {
	// Arrange
	tests := map[string]struct {
		givenCache           PropCache
		givenTimestamp       time.Time
		expectedSuccessCount int
		expectedErrs         error
	}{
		"should refresh all entries even if none are expired": {
			givenCache: PropCache{
				expireAfter: 1000 * time.Millisecond,
				entries: map[string]CacheEntry{
					"test1": {UpdatedAtMillis: 1, Props: make(map[string]string)},
					"test2": {UpdatedAtMillis: 1, Props: make(map[string]string)},
				},
				entryRefreshFn: testingEntryRefreshFn,
			},
			givenTimestamp:       time.UnixMilli(1),
			expectedSuccessCount: 2,
		},
		"should refresh all entries even if 1 entry has expired": {
			givenCache: PropCache{
				expireAfter: 1 * time.Millisecond,
				entries: map[string]CacheEntry{
					"test1": {UpdatedAtMillis: 5, Props: make(map[string]string)},
					"test2": {UpdatedAtMillis: 10, Props: make(map[string]string)},
				},
				entryRefreshFn: testingEntryRefreshFn,
			},
			givenTimestamp:       time.UnixMilli(6),
			expectedSuccessCount: 2,
		},
		"should refresh all entries even if 2 entries have expired": {
			givenCache: PropCache{
				expireAfter: 1 * time.Millisecond,
				entries: map[string]CacheEntry{
					"test1": {UpdatedAtMillis: 5, Props: make(map[string]string)},
					"test2": {UpdatedAtMillis: 10, Props: make(map[string]string)},
					"test3": {UpdatedAtMillis: 15, Props: make(map[string]string)},
				},
				entryRefreshFn: testingEntryRefreshFn,
			},
			givenTimestamp:       time.UnixMilli(11),
			expectedSuccessCount: 3,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			count, errs := test.givenCache.RefreshCacheForce(context.Background())

			// Assert
			assert.Equal(t, test.expectedErrs, errs)
			assert.Equal(t, test.expectedSuccessCount, count, fmt.Sprintf("the number of refreshed entries didn't match expected: %+v", test.givenCache))
		})
	}
}
