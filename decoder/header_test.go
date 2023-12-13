package decoder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ChaosNyaruko/ondict/decoder"
)

func Test_Decode(t *testing.T) {
	m := decoder.MDict{}
	assert.Nil(t, m.Decode("Longman Dictionary of Contemporary English.mdx"))
}
