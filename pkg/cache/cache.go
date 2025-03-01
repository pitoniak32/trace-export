package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	myOtel "github.com/pitoniak32/trace-export/pkg/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer(myOtel.SERVICE_TRACER_NAME)
)

type CacheEntry struct {
	UpdatedAtMillis int64
	Props           map[string]string
}

type PropCache struct {
	// the number of seconds that need to pass since the UpdatedAt of an entry for it to be expired
	expireAfter time.Duration
	entries     map[string]CacheEntry
	// will be called for each entry that is considered expired when a cache refresh is requested
	entryRefreshFn func(ctx context.Context, name string, entry *CacheEntry) error
}

func NewPropCache() PropCache {
	c := PropCache{
		expireAfter: 10 * time.Second,
		entries:     make(map[string]CacheEntry),
		entryRefreshFn: func(ctx context.Context, name string, entry *CacheEntry) error {
			span := trace.SpanFromContext(ctx)

			// Create a timeout to make sure that the refresh doesnt hang forever
			ctxTimeout, cancelRefresh := context.WithTimeout(ctx, 5*time.Second)
			defer cancelRefresh()

			req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, "https://jsonplaceholder.typicode.com/todos/1", nil)
			if err != nil {
				fmt.Println("Error creating request:", err)
				return err
			}

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			fmt.Println("Response:", string(body))

			if name == "trace-export" {
				err = fmt.Errorf("ran into error updating repo '%s'", name)
				span.SetStatus(codes.Error, err.Error())
			} else {
				entry.UpdatedAtMillis = time.Now().UnixMilli()
			}

			return err
		},
	}
	return c
}

func (c *PropCache) InsertMap(inEntries map[string]CacheEntry) {
	for name, entry := range inEntries {
		c.entries[name] = entry
	}
}

func (c *PropCache) Insert(name string, entry CacheEntry) {
	c.entries[name] = entry
}

func (c *PropCache) GetProps(repoName string) map[string]string {
	return c.entries[repoName].Props
}

func runAndWait(inverval time.Duration, fn func()) {
	earlier := time.Now()
	fn()
	diff := time.Since(earlier)

	<-time.After(inverval - diff)
}

func (c *PropCache) ScheduleRefresh(ctx context.Context, interval time.Duration) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				fmt.Println(ctx.Err().Error())
				return
			default:
				runAndWait(interval, func() {
					ctx, span := tracer.Start(ctx, "ScheduledCacheRefresh", trace.WithAttributes(attribute.Int64("refresh.schedule.interval.ms", int64(interval.Milliseconds()))))
					defer span.End()

					var joined interface{ Unwrap() []error }

					successCount, errs := c.RefreshCacheExpiredAt(ctx, time.Now().Add(10_001*time.Millisecond))

					failedCount := 0

					if errors.As(errs, &joined) {
						joinedErrs := joined.Unwrap()
						for _, err := range joinedErrs {
							fmt.Printf("[REFRESH]: Error - failed to refresh cache entry: %s\n", err)
						}
						failedCount = len(joinedErrs)
					}

					span.SetAttributes(
						attribute.Int("refresh.total.succeeded", successCount),
						attribute.Int("refresh.total.failed", failedCount),
					)

					fmt.Printf("[REFRESH]: Summary - success: %d, failed: %d\n", successCount, failedCount)
				})
			}
		}
	}()
}

// unixMillis - the time in unixMillis that should be used to determine expired entries
func (c *PropCache) RefreshCacheExpiredAt(ctx context.Context, expireAfter time.Time) (int, error) {
	return c.refreshCache(ctx, func(ctx context.Context, name string, entry CacheEntry) bool {
		elapsed := expireAfter.UnixMilli() - entry.UpdatedAtMillis
		return elapsed >= c.expireAfter.Milliseconds()
	})
}

// will refresh all cache entries even if they have not expired
func (c *PropCache) RefreshCacheForce(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(ctx, "RefreshCacheForce")
	defer span.End()

	return c.refreshCache(ctx, func(ctx context.Context, _ string, _ CacheEntry) bool {
		return true
	})
}

func (c *PropCache) refreshCache(ctx context.Context, isExpired func(ctx context.Context, name string, entry CacheEntry) bool) (int, error) {
	var wg sync.WaitGroup
	ch := make(chan error)

	for name, entry := range c.entries {
		ctx, span := tracer.Start(ctx, name)
		defer span.End()

		exp := isExpired(ctx, name, entry)

		span.SetAttributes(
			attribute.Bool("is_expired", exp),
		)

		if exp {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ch <- c.entryRefreshFn(ctx, name, &entry)
			}()
		}
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var errs error
	successCount := 0
	for err := range ch {
		if err != nil {
			errs = errors.Join(errs, err)
		} else {
			successCount += 1
		}
	}

	return successCount, errs
}
