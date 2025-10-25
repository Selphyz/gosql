package provider

import (
	"fmt"
	"strings"
	"sync"
)

var (
	mu        sync.RWMutex
	providers = make(map[string]SQLProvider)
	schemeMap = make(map[string]string)
)

// Register binds a provider implementation to one or more lookup keys.
func Register(name string, p SQLProvider, schemes ...string) {
	lowerName := strings.ToLower(name)

	mu.Lock()
	defer mu.Unlock()

	providers[lowerName] = p
	for _, scheme := range schemes {
		if scheme == "" {
			continue
		}
		schemeMap[strings.ToLower(scheme)] = lowerName
	}
}

// ByName retrieves a provider by its registered name.
func ByName(name string) (SQLProvider, error) {
	mu.RLock()
	defer mu.RUnlock()

	if p, ok := providers[strings.ToLower(name)]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("provider %q not registered", name)
}

// ByScheme retrieves a provider registered for a particular URL scheme.
func ByScheme(scheme string) (SQLProvider, error) {
	mu.RLock()
	defer mu.RUnlock()

	name, ok := schemeMap[strings.ToLower(scheme)]
	if !ok {
		return nil, fmt.Errorf("no provider registered for scheme %q", scheme)
	}
	p, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not registered", name)
	}
	return p, nil
}

// Resolve selects a provider either by explicit name or inferred from a connection string scheme.
func Resolve(providerName, connection string) (SQLProvider, error) {
	if providerName != "" {
		return ByName(providerName)
	}

	if idx := strings.Index(connection, "://"); idx > 0 {
		return ByScheme(connection[:idx])
	}

	return nil, fmt.Errorf("could not infer provider from connection string %q", connection)
}
