package conn

import (
	"fmt"
	"strings"

	"gosql/internal/provider"
)

// Normalize resolves the provider and returns a driver DSN alongside the database name.
func Normalize(explicitProvider, input string) (provider.SQLProvider, string, string, error) {
	var prov provider.SQLProvider
	var err error

	if explicitProvider != "" {
		prov, err = provider.ByName(explicitProvider)
		if err != nil {
			return nil, "", "", err
		}
	} else {
		prov, err = detectProvider(input)
		if err != nil {
			return nil, "", "", err
		}
	}

	dsn, dbName, err := prov.ParseConnection(input)
	if err != nil {
		return nil, "", "", err
	}
	return prov, dsn, dbName, nil
}

func detectProvider(input string) (provider.SQLProvider, error) {
	if idx := strings.Index(input, "://"); idx > 0 {
		return provider.ByScheme(input[:idx])
	}

	// Heuristic for MySQL DSNs (user@protocol(address)/db or mini variants).
	if strings.Contains(input, "@tcp(") || strings.Contains(input, "@unix(") || strings.Contains(input, "@/") {
		if p, err := provider.ByName("mysql"); err == nil {
			return p, nil
		}
	}

	// Heuristic for SQL Server ADO connection strings (key=value;...)
	lower := strings.ToLower(input)
	if (strings.Contains(lower, "server=") || strings.Contains(lower, "data source=")) &&
		(strings.Contains(lower, "database=") || strings.Contains(lower, "initial catalog=")) {
		if p, err := provider.ByName("sqlserver"); err == nil {
			return p, nil
		}
	}

	// Heuristic for Oracle ezconnect strings (user/pass@host:port/service)
	if strings.Contains(input, "@") && strings.Contains(input, "/") && !strings.Contains(input, "://") {
		// Could be Oracle ezconnect format
		if p, err := provider.ByName("oracle"); err == nil {
			return p, nil
		}
	}

	return nil, fmt.Errorf("could not detect provider from connection string %q; specify --database or --provider", input)
}
