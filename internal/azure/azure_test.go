package azure

import (
	"errors"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{"resource code", `az aks show: (ResourceNotFound) The Resource 'Microsoft.ContainerService/managedClusters/x' under resource group 'rg' was not found.`, true},
		{"resource group code", `az aks show: (ResourceGroupNotFound) Resource group 'rg' could not be found.`, true},
		{"lowercase code", "error code: resourcenotfound", true},
		{"generic not found", "connection not found blah", false},
		{"auth error", "az login required", false},
		{"loose was not found", "the thing was not found", false},
		{"loose could not be found", "it could not be found", false},
		{"empty", "", false},
		{"whitespace", "   ", false},
	}
	for _, c := range cases {
		if got := isNotFound(errors.New(c.msg)); got != c.want {
			t.Errorf("%s: isNotFound(%q) = %v, want %v", c.name, c.msg, got, c.want)
		}
	}
}
