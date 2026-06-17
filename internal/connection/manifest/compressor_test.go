package manifest

import (
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZipCompressor_Compress(t *testing.T) {
	t.Parallel()

	input := map[string][]byte{
		faker.Word(): []byte(faker.Word()),
	}

	target := NewZipCompressor()

	actual, err := target.Compress(t.Context(), input)
	require.NoError(t, err)
	assert.Greater(t, actual.Len(), 130)
}
