package downloader

import (
	"sync"
)

type URLsPool struct {
	m       *sync.RWMutex
	content map[string]bool
}

func CreatePool(mu *sync.RWMutex) *URLsPool {
	return &URLsPool{
		m:       mu,
		content: make(map[string]bool, 1),
	}
}

func (p *URLsPool) Exist(url string) bool {
	value, ok := p.content[url]
	if ok {
		return value && ok
	} else {
		return ok
	}
}

func (p *URLsPool) Add(url string) {
	p.m.RLock()
	p.content[url] = true
	p.m.RUnlock()
}
