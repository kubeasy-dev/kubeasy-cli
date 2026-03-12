package deployer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudProviderKindBinaryURL_LinuxAMD64(t *testing.T) {
	url := cloudProviderKindBinaryURLForPlatform("linux", "amd64")
	assert.Contains(t, url, "cloud-provider-kind_")
	assert.Contains(t, url, "_linux_amd64.tar.gz")
	assert.True(t, strings.HasPrefix(url, "https://"), "URL should be HTTPS")
}

func TestCloudProviderKindBinaryURL_DarwinARM64(t *testing.T) {
	url := cloudProviderKindBinaryURLForPlatform("darwin", "arm64")
	assert.Contains(t, url, "cloud-provider-kind_")
	assert.Contains(t, url, "_darwin_arm64.tar.gz")
	assert.True(t, strings.HasPrefix(url, "https://"), "URL should be HTTPS")
}

func TestCloudProviderKindBinaryURL_VersionStripping(t *testing.T) {
	url := cloudProviderKindBinaryURLForPlatform("linux", "amd64")
	// CloudProviderKindVersion starts with "v", e.g. "v0.10.0"
	// The filename portion must NOT contain the "v" prefix
	require.NotEmpty(t, CloudProviderKindVersion, "CloudProviderKindVersion should be set")
	versionWithoutV := strings.TrimPrefix(CloudProviderKindVersion, "v")

	// The URL path (tag) should contain the full version WITH "v"
	assert.Contains(t, url, "/"+CloudProviderKindVersion+"/",
		"URL path should contain version with 'v' prefix as the release tag")

	// The filename should contain the version WITHOUT "v"
	assert.Contains(t, url, "cloud-provider-kind_"+versionWithoutV+"_",
		"filename in URL should contain version WITHOUT 'v' prefix")

	// The "v" prefix should not appear in the filename
	assert.NotContains(t, url, "cloud-provider-kind_v",
		"filename in URL should NOT start the version with 'v'")
}
