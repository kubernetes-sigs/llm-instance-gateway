package scheduling

import (
	"fmt"
	"sync"
	"time"
)

type TokenCache struct {
	AdapterMap sync.Map
	TTL        int64
}
type TokenRepsonseData struct {
	Time       int64
	TokenCount int
}

func CreateNewTokenCache(ttl int64) *TokenCache {
	return &TokenCache{AdapterMap: sync.Map{}, TTL: ttl}
}

func (c *TokenCache) IsFairRequest(adapter string) bool {
	current := time.Now().Unix()
	adapterTokens := 0
	total := 0
	adapterCount := 0
	fmt.Println("Liveness", adapter)
	c.AdapterMap.Range(func(k, v any) bool {
		local_sum := 0
		for _, entry := range v.([]TokenRepsonseData) {
			if entry.Time > int64(current-c.TTL) {
				local_sum += entry.TokenCount
			}
		}
		if local_sum > 0 {
			adapterCount = adapterCount + 1
		}
		if k.(string) == adapter {
			adapterTokens = local_sum
		} else {
			fmt.Println("k adapter:", k, adapter)
		}
		total += local_sum
		return true
	})
	if adapterCount == 0 {
		fmt.Println("No adapter")
		return true
	}
	fairShare := total / adapterCount
	fmt.Println("adapter Tokens: %+v; Fair share %+v", adapterTokens, fairShare)
	if adapterTokens > fairShare {
		return false
	}
	return true
}

func (c *TokenCache) StoreResponseInfo(model string, currentTime int64, totalTokens int) {
	tokenData := TokenRepsonseData{Time: currentTime, TokenCount: totalTokens}
	if v, ok := c.AdapterMap.Load(model); ok {
		val := v.([]TokenRepsonseData)
		newArr := []TokenRepsonseData{}
		for _, entry := range val {
			if entry.Time >= int64(currentTime-c.TTL) {
				newArr = append(newArr, entry)
			}
		}
		c.AdapterMap.Store(model, append(newArr, tokenData))
	} else {
		c.AdapterMap.Store(model, []TokenRepsonseData{tokenData})
	}
}
