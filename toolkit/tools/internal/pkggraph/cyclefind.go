// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package pkggraph

import (
	"gonum.org/v1/gonum/graph"

	"microsoft.com/pkggen/internal/logger"
)

const (
	invalid = iota
	unvisited
	inProgress
	done
)

type bfsData struct {
	state  map[int64]int
	parent map[int64]int64
	cycle  []int64
}

func createCycle(g *PkgGraph, metaData *bfsData, start, end int64) {
	metaData.cycle = append(metaData.cycle, end)
	logger.Log.Errorf("%s needed by %s", g.Node(end).(*PkgNode).FriendlyName(), g.Node(start).(*PkgNode).FriendlyName())
	for end != start {
		metaData.cycle = append(metaData.cycle, start)
		logger.Log.Errorf("%s needed by %s", g.Node(start).(*PkgNode).FriendlyName(), g.Node(metaData.parent[start]).(*PkgNode).FriendlyName())
		start = metaData.parent[start]
	}
}

func bfs(g *PkgGraph, u int64, metaData *bfsData) (foundCycle bool) {
	node := g.Node(u).(*PkgNode)
	logger.Log.Errorf(node.String())
	if metaData.state[u] != unvisited {
		logger.Log.Panicf("Node %d is in a bad state! (%d)", u, metaData.state[u])
	}

	metaData.state[u] = inProgress

	foundCycle = false
	for _, neighbor := range graph.NodesOf(g.From(u)) {
		v := neighbor.ID()
		_, exists := metaData.state[v]
		if !exists {
			metaData.state[v] = unvisited
		}

		switch metaData.state[v] {
		case done:
			continue

		case unvisited:
			metaData.parent[v] = u
			foundCycle = bfs(g, v, metaData)
			if foundCycle {
				return
			}
		case inProgress:
			logger.Log.Error("Found cycle!")
			createCycle(g, metaData, u, v)
			foundCycle = true
			return
		default:
			logger.Log.Panicf("Node %d is in a bad state! (%d)", v, metaData.state[v])
		}
	}

	metaData.state[u] = done
	return
}

// FindAnyDirectedCycle returns any single cycle in the graph, if one exists.
func (g *PkgGraph) FindAnyDirectedCycle() (nodes []PkgNode, err error) {
	bfsData := bfsData{
		make(map[int64]int),
		make(map[int64]int64),
		make([]int64, 0),
	}

	workingGraph, err := g.DeepCopy()
	if err != nil {
		return
	}

	rootNode, err := workingGraph.AddGoalNode("BFSRoot", nil, false)
	if err != nil {
		return
	}
	bfsData.parent[rootNode.ID()] = -1
	bfsData.state[rootNode.ID()] = unvisited

	foundCycle := bfs(workingGraph, rootNode.ID(), &bfsData)

	if foundCycle {
		for _, id := range bfsData.cycle {
			nodes = append(nodes, *g.Node(id).(*PkgNode))
		}
	}

	return
}
