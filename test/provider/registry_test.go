package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gosql/internal/provider"
	_ "gosql/internal/provider/mysql"
	_ "gosql/test/testutil/fakeprovider"
)

func TestRegister_ByName(t *testing.T) {
	// The fake provider should be registered by init()
	prov, err := provider.ByName("fake")
	require.NoError(t, err)
	assert.NotNil(t, prov)
	assert.Equal(t, "sqlmock", prov.DriverName())
}

func TestByName_CaseInsensitive(t *testing.T) {
	prov1, err1 := provider.ByName("fake")
	require.NoError(t, err1)

	prov2, err2 := provider.ByName("FAKE")
	require.NoError(t, err2)

	assert.Equal(t, prov1, prov2)
}

func TestByName_NotRegistered(t *testing.T) {
	prov, err := provider.ByName("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, prov)
	assert.Contains(t, err.Error(), "not registered")
}

func TestByScheme_Success(t *testing.T) {
	// fake provider is registered with scheme "fake"
	prov, err := provider.ByScheme("fake")
	require.NoError(t, err)
	assert.NotNil(t, prov)
}

func TestByScheme_CaseInsensitive(t *testing.T) {
	prov1, err1 := provider.ByScheme("fake")
	require.NoError(t, err1)

	prov2, err2 := provider.ByScheme("FAKE")
	require.NoError(t, err2)

	assert.Equal(t, prov1, prov2)
}

func TestByScheme_NotRegistered(t *testing.T) {
	prov, err := provider.ByScheme("unknownscheme")
	assert.Error(t, err)
	assert.Nil(t, prov)
	assert.Contains(t, err.Error(), "no provider registered for scheme")
}

func TestResolve_ExplicitName(t *testing.T) {
	prov, err := provider.Resolve("fake", "ignored-connection-string")
	require.NoError(t, err)
	assert.NotNil(t, prov)
}

func TestResolve_ByScheme(t *testing.T) {
	prov, err := provider.Resolve("", "mysql://localhost/db")
	require.NoError(t, err)
	assert.NotNil(t, prov)
	assert.Equal(t, "mysql", prov.DriverName())
}

func TestResolve_NoProvider(t *testing.T) {
	prov, err := provider.Resolve("", "no-scheme-or-provider")
	assert.Error(t, err)
	assert.Nil(t, prov)
	assert.Contains(t, err.Error(), "could not infer provider")
}

func TestResolve_UnknownScheme(t *testing.T) {
	prov, err := provider.Resolve("", "unknownscheme://localhost/db")
	assert.Error(t, err)
	assert.Nil(t, prov)
}

func TestResolve_ExplicitNameTakesPrecedence(t *testing.T) {
	// Even with a URL that has a scheme, explicit name should be used
	prov, err := provider.Resolve("fake", "mysql://localhost/db")
	require.NoError(t, err)
	assert.NotNil(t, prov)
	assert.Equal(t, "sqlmock", prov.DriverName())
}