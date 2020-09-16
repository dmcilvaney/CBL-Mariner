package pkggraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Validate the test graph is well formed
func TestBFSFindCycle(t *testing.T) {
	g, err := buildTestGraphHelper()
	assert.NoError(t, err)
	assert.NotNil(t, g)

	addEdgeHelper(g, *pkgCBuild, *pkgARun)

	// Check the correctness of the disconnected components rooted in pkgARun, and pkgC2Run
	checkTestGraph(t, g)

	cycle, err := g.FindAnyDirectedCycle()
	assert.NoError(t, err)
	assert.NotNil(t, cycle)
	assert.Equal(t, 6, len(cycle))
}

func TestBFSNoCycle(t *testing.T) {
	g, err := buildTestGraphHelper()
	assert.NoError(t, err)
	assert.NotNil(t, g)

	// Check the correctness of the disconnected components rooted in pkgARun, and pkgC2Run
	checkTestGraph(t, g)

	cycle, err := g.FindAnyDirectedCycle()
	assert.NoError(t, err)
	assert.Nil(t, cycle)
}
