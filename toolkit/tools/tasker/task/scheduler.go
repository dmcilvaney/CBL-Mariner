// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package task

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

type SchedGraph struct {
	*simple.DirectedGraph
}

type SchedNode struct {
	*simple.Node
	taskRef Tasker
}

func (n SchedNode) DOTID() string {
	return n.taskRef.ID()
}

const (
	NoSelfCycle    = false
	AllowSelfCycle = true
)

// pick a color based on the task's ID. Need to use regex to match the task ID.
// TODO: make this not suck
func (n SchedNode) dotColor() string {
	id := n.taskRef.ID()
	dirtLevel := n.taskRef.DirtyLevel()
	// Scale colours up to dirt level 3
	if strings.Contains(id, "BUILDSPEC") {
		return []string{"slateblue", "skyblue", "slategray1"}[dirtLevel%3]

	} else if strings.Contains(id, "BUILDSRPM") {
		return []string{"darkseagreen3", "darkolivegreen1", "darkseagreen1"}[dirtLevel%3]

	} else if strings.Contains(id, "CACHE") {
		return []string{"darkred", "firebrick3", "firebrick1"}[dirtLevel%3]

	} else if strings.Contains(id, "CAP") {
		return []string{"darkgoldenrod3", "darkgoldenrod2", "darkgoldenrod1"}[dirtLevel%3]
	}
	return ""
}

func (n SchedNode) edgeColor() (color, size string) {
	task := n.taskRef
	if task.IsDone() {
		return "green", "8"
	}
	return "black", "1"
}

func (n SchedNode) Attributes() []encoding.Attribute {
	e := []encoding.Attribute{
		{Key: "label", Value: n.taskRef.Name()},
	}
	color := n.dotColor()
	if color != "" {
		e = append(e, encoding.Attribute{Key: "fillcolor", Value: color})
		e = append(e, encoding.Attribute{Key: "style", Value: "filled"})
	}
	edgeColor, edgeSize := n.edgeColor()
	if edgeColor != "" {
		e = append(e, encoding.Attribute{Key: "color", Value: edgeColor})
	}
	if edgeSize != "" {
		e = append(e, encoding.Attribute{Key: "penwidth", Value: edgeSize})
	}
	return e
}

type Scheduler struct {
	tasks           []*SchedNode
	metadataLock    sync.RWMutex
	fileLock        sync.Mutex
	runSequential   bool
	graph           SchedGraph
	rootNode        *SchedNode
	doPrintProgress bool
}

func NewScheduler(runSequential bool) *Scheduler {
	s := &Scheduler{
		//tasks:         []Tasker{},
		metadataLock:    sync.RWMutex{},
		fileLock:        sync.Mutex{},
		runSequential:   runSequential,
		graph:           SchedGraph{simple.NewDirectedGraph()},
		doPrintProgress: false,
	}
	// Add a dummpy node to the graph
	baseNodeId := s.graph.NewNode().ID()
	s.rootNode = &SchedNode{(*simple.Node)(&baseNodeId), &BasicTask{
		basicTaskID:   "graph_root_node",
		basicTaskName: "graph_root_node",
		dirtyLevel:    0,
	}}
	s.graph.AddNode(s.rootNode)
	s.tasks = append(s.tasks, s.rootNode)
	return s
}

func (s *Scheduler) addTaskToGraph(parent, child Tasker) {
	// Make new child node
	newCNodeID := s.graph.NewNode().ID()
	cNode := &SchedNode{(*simple.Node)(&newCNodeID), child}

	s.tasks = append(s.tasks, cNode)

	if parent != nil {
		// Find the existing parent node
		pNode := s.getTaskInternalNode(parent)
		// Add edge
		s.graph.SetEdge(s.graph.NewEdge(pNode, cNode))
	} else {
		// Add orphaned node
		s.graph.AddNode(cNode)
	}
}

func (s *Scheduler) willAddNewCycle(parent, child Tasker, allowSelfCycle bool) bool {
	// Get the existing nodes
	pNode := s.getTaskInternalNode(parent)
	cNode := s.getTaskInternalNode(child)

	// Check if self cycle
	if pNode.ID() == cNode.ID() {
		//TODO DETECT PACKAGE SELF CYCLES
		parent.TLog(logrus.InfoLevel, "Self cycle detected for %s", parent.ID())
		return !allowSelfCycle
	}

	// Check for cycle if we added this edge
	if s.graph.HasEdgeFromTo(pNode.ID(), cNode.ID()) {
		// If the edge already exists, its probably fine
		return false
	}

	s.graph.SetEdge(s.graph.NewEdge(pNode, cNode))
	defer s.graph.RemoveEdge(pNode.ID(), cNode.ID())

	// Check for cycle
	cycles := topo.DirectedCyclesIn(s.graph)
	return len(cycles) > 0
}

func (s *Scheduler) addEdge(parent, child *SchedNode) {
	if parent.ID() == child.ID() || s.graph.HasEdgeFromTo(parent.ID(), child.ID()) {
		return
	} else {
		s.graph.SetEdge(s.graph.NewEdge(parent, child))
	}
}

// Returns nil if the task can't be added due to cycle
func (s *Scheduler) AddTask(parent, newTask Tasker, allowSelfCycle bool) Tasker {
	if !s.runSequential {
		s.metadataLock.Lock()
		defer s.metadataLock.Unlock()
	}
	if parent == nil {
		parent = s.rootNode.taskRef
	}

	if newTask.DirtyLevel() > buildconfig.CurrentBuildConfig.MaxDirt {
		newTask.TLog(logrus.FatalLevel, "Task %s has a dirt level greater than the max dirt level", newTask.ID())
		return nil
	}

	existingNewTaskNode := s.getTaskInternalNode(newTask)
	if existingNewTaskNode != nil {
		parent.TLog(logrus.InfoLevel, "Task %s already exists in the scheduler", newTask.ID())
		newTask = existingNewTaskNode.taskRef

		//Check cycle, we must return error if cycle is detected
		if s.willAddNewCycle(parent, newTask, allowSelfCycle) {
			parent.TLog(logrus.WarnLevel, "Cycle detected between %s and %s", parent.ID(), newTask.ID())
			return nil
		} else {
			// Add the task to the graph
			s.addEdge(s.getTaskInternalNode(parent), existingNewTaskNode)
		}

	} else {
		parent.TLog(logrus.InfoLevel, "Adding task %s to the scheduler", newTask.ID())
		s.addTaskToGraph(parent, newTask)

		newTask.registerWithScheduler(s.AddTask)
		if s.runSequential {
			newTask.Execute()
		} else {
			go newTask.Execute()
		}
	}

	return newTask

}

func (s *Scheduler) GetTask(task Tasker) Tasker {
	s.metadataLock.RLock()
	defer s.metadataLock.RUnlock()
	return s.getTaskInternalNode(task).taskRef
}

func (s *Scheduler) getTaskInternalNode(prospectiveTask Tasker) *SchedNode {
	for _, t := range s.tasks {
		if t.taskRef.ID() == prospectiveTask.ID() && t.taskRef.DirtyLevel() == prospectiveTask.DirtyLevel() {
			return t
		}
	}
	return nil
}

func (s *Scheduler) StartProgressPrinter() {
	s.metadataLock.RLock()
	defer s.metadataLock.RUnlock()
	if s.doPrintProgress {
		return
	}
	s.doPrintProgress = true

	go func() {
		for s.doPrintProgress {
			s.metadataLock.RLock()
			total := len(s.tasks)
			done := 0
			for _, t := range s.tasks {
				if t.taskRef.IsDone() {
					done++
				}
			}
			percent := (float64(done) / float64(total)) * 100

			logger.Log.Infof("Progress: %d/%d tasks done (%.2f%%)", done, total, percent)
			s.metadataLock.RUnlock()

			// Sleep for a bit
			<-time.After(5 * time.Second)
		}
	}()
}

func (s *Scheduler) StopProgressPrinter() {
	s.metadataLock.Lock()
	defer s.metadataLock.Unlock()
	s.doPrintProgress = false
}

func (s *Scheduler) Done() bool {
	s.metadataLock.RLock()
	defer s.metadataLock.RUnlock()

	for _, t := range s.tasks {
		if !t.taskRef.IsDone() {
			return false
		}
	}
	return true
}

// WriteDOTGraph serializes a graph into a DOT formatted object
func (s *Scheduler) WriteDOTGraph(outputFull, outputClean io.Writer) (err error) {
	s.metadataLock.RLock()
	s.fileLock.Lock()
	defer s.metadataLock.RUnlock()
	defer s.fileLock.Unlock()

	if outputFull != nil {
		bytesFull, err := dot.Marshal(s.graph, "scheduler", "", "")
		if err != nil {
			return err
		}
		_, err = outputFull.Write(bytesFull)
		if err != nil {
			return err
		}
	}

	// We want to hack up the graph to make it look nice
	gCopy := SchedGraph{simple.NewDirectedGraph()}
	for _, n := range graph.NodesOf(s.graph.Nodes()) {
		// Get the node
		node := n.(*SchedNode)
		gCopy.AddNode(node)
	}
	for _, e := range graph.EdgesOf(s.graph.Edges()) {
		gCopy.SetEdge(e)
	}
	// Remove any node that is SPEC_DATA_DB
	for _, n := range graph.NodesOf(gCopy.Nodes()) {
		node := n.(*SchedNode)
		if node.taskRef.ID() == "SPEC_DATA_DB" {
			gCopy.RemoveNode(node.ID())
		}
	}
	// Remove any node that is a azl-toolchain capability with dirt maxdirt
	for _, n := range graph.NodesOf(gCopy.Nodes()) {
		node := n.(*SchedNode)
		id := node.taskRef.ID()
		dirt := node.taskRef.DirtyLevel()
		if dirt == buildconfig.CurrentBuildConfig.MaxDirt && strings.Contains(id, "CAP") && strings.Contains(id, "azl-toolchain") {
			gCopy.RemoveNode(node.ID())
		}
	}
	// remove all nodes that have no parents, except the root node. Repeat until no more nodes are removed
	for didRemove := true; didRemove; {
		didRemove = false
		for _, n := range graph.NodesOf(gCopy.Nodes()) {
			node := n.(*SchedNode)
			if gCopy.To(node.ID()).Len() == 0 && node.ID() != s.rootNode.ID() {
				gCopy.RemoveNode(node.ID())
				didRemove = true
			}
		}
	}

	if outputClean != nil {
		bytesClean, err := dot.Marshal(gCopy, "scheduler", "", "")
		if err != nil {
			return err
		}
		_, err = outputClean.Write(bytesClean)
		if err != nil {
			return err
		}
	}

	return
}
