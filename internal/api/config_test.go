package api_test

import (
	"testing"
	"time"

	"OzonBankTestTask/internal/api"
	"OzonBankTestTask/internal/graphql/mocks"

	"github.com/stretchr/testify/require"
)

func TestHandlerRejectsInvalidConfiguration(t *testing.T) {
	_, err := api.NewHandler(nil, nil, nil, api.Config{})
	require.Error(t, err)
	for _, config := range []api.Config{{MaxBodyBytes: -1}, {RequestTimeout: -time.Second}, {MaxDepth: -1}, {MaxDepth: 101}, {MaxComplexity: -1}, {MaxComplexity: 1_000_001}} {
		_, posts, comments := newHandler(t, api.Config{})
		_, err := api.NewHandler(posts, comments, mocks.NewSubscriptionUseCase(t), config)
		require.Error(t, err)
	}
}
