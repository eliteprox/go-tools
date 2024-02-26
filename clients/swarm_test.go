package clients

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSwarmClientAPIKey(t *testing.T) {
	apiKey := ""
	apiSecret := ""
	swarmClient := NewSwarmClientAPIKey(apiKey, apiSecret)

	assert.NotNil(t, swarmClient)

}
