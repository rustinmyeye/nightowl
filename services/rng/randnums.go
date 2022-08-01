package rng

import (
	"sync"
)

type ErgBlockRandNum struct {
	mu       sync.Mutex
	randNums map[string]string
}

var allErgBlockRandNums *ErgBlockRandNum

func init() {
	
	allErgBlockRandNums = &ErgBlockRandNum{
		randNums: make(map[string]string),
	}
}

func GetRandHashMap() *ErgBlockRandNum {
	return allErgBlockRandNums
}

func (e *ErgBlockRandNum) Get(key string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	val, ok := e.randNums[key]
	return val, ok
}

func (e *ErgBlockRandNum) Delete(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.randNums, key)
}