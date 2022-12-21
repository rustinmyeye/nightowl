package rng

import (
	"sync"
)

type DrandNum struct {
	mu       sync.Mutex
	randNums map[string]string
}

var allDrandNums *DrandNum

func init() {
	
	allDrandNums = &DrandNum{
		randNums: make(map[string]string),
	}
}

func GetRandHashMap() *DrandNum {
	return allDrandNums
}

func (e *DrandNum) Get(key string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	val, ok := e.randNums[key]
	return val, ok
}

func (e *DrandNum) Delete(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.randNums, key)
}