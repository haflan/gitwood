package main

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type cacheGenResult struct {
	notifier chan struct{}
	data     any
	err      error
}

var dataCache = struct {
	cacheGenResultMap map[string]*cacheGenResult
	sync.RWMutex
}{
	cacheGenResultMap: make(map[string]*cacheGenResult),
}

func generateAndCacheData(key string, generate func() (any, error)) {
	// Checking if already exists, and creating new if not, must be done before launching a new go routine,
	// otherwise there's a risk that the handler calling this function finishes before the cache entry can even be made.
	dataCache.Lock()
	_, exists := dataCache.cacheGenResultMap[key]
	if exists {
		dataCache.Unlock()
		return
	}
	dataCache.cacheGenResultMap[key] = &cacheGenResult{notifier: make(chan struct{})}
	dataCache.Unlock()
	go func() {
		// Generating data might take time, so this should not block the dataCache
		data, err := generate()
		// Store the result or error
		dataCache.Lock()
		var c chan struct{}
		if err != nil {
			dataCache.cacheGenResultMap[key].err = err
		} else {
			dataCache.cacheGenResultMap[key].data = data
		}
		// Remove the channel from the mapping...
		c = dataCache.cacheGenResultMap[key].notifier
		dataCache.cacheGenResultMap[key].notifier = nil
		dataCache.Unlock()
		// ...and try to notify it.
		if c != nil {
			// NOTE: A proper PubSub is needed for this to scale: https://eli.thegreenplace.net/2020/pubsub-using-channels-in-go/,
			// but I doubt that will be a problem with one user :o)
			select {
			case c <- struct{}{}:
			default:
			}
		}
	}()
}

var ErrCacheWaitTimeout = errors.New("timed out waiting for data")
var ErrCacheNotFound = errors.New("cache not found")

func getCachedData(key string, waitFor time.Duration) (any, error) {
	dataCache.RLock()
	result, ok := dataCache.cacheGenResultMap[key]
	dataCache.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %v", ErrCacheNotFound, key)
	}
	if result.notifier != nil {
		select {
		case <-time.After(waitFor):
			return nil, ErrCacheWaitTimeout
		case <-result.notifier:
		}
	}
	return result.data, result.err
}

func (pc *PageContext) requireCachedTodos(w http.ResponseWriter, projectPath, commitHash string, waitFor time.Duration) []TodoDesc {
	atodos, err := getCachedData(commitCacheKey(projectPath, commitHash, "todo"), waitFor)
	if err != nil {
		if errors.Is(err, ErrCacheWaitTimeout) {
			// TODO [timeout_error_page]: New page with appropriate HTTP code (202?) when waiting for cache times out.
			// The page can include a `<meta http-equiv="refresh" content="1">` to auto-refresh after a second.
			pc.errorPageServer(w, "loading todos takes time - please try again in a second", err)
		} else {
			pc.errorPageServer(w, "failed to find todos", err)
		}
		return nil
	}
	todos, ok := atodos.([]TodoDesc)
	if !ok {
		pc.errorPageServer(w, "unexpected error when loading todos", err)
		return nil
	}
	return todos
}
