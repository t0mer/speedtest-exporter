package runner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t0mer/speedtest-exporter/internal/model"
	"github.com/t0mer/speedtest-exporter/internal/runner"
)

func TestGoRunnerEngine(t *testing.T) {
	r := runner.NewGoRunner()
	assert.Equal(t, model.EngineGo, r.Engine())
}
