// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
	"microsoft.com/pkggen/internal/exe"
	"microsoft.com/pkggen/internal/logger"
	"microsoft.com/pkggen/internal/pkggraph"
	"microsoft.com/pkggen/internal/pkgjson"
)

var (
	app    = kingpin.New("grapher", "Dependency graph generation tool")
	input  = exe.InputFlag(app, "Input json listing all local SRPMs")
	output = exe.OutputFlag(app, "Output file to export the graph to")

	logFile          = exe.LogFileFlag(app)
	logLevel         = exe.LogLevelFlag(app)
	strictGoals      = app.Flag("strict-goals", "Don't allow missing goal packages").Bool()
	strictUnresolved = app.Flag("strict-unresolved", "Don't allow missing unresolved packages").Bool()
	targetArch       = app.Flag("target-arch", "Cross compile target arch").String()

	depGraph = pkggraph.NewPkgGraph()
)

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	var (
		err       error
		buildArch = "x86_64"
	)
	logger.InitBestEffort(*logFile, *logLevel)

	if targetArch == nil || len(*targetArch) == 0 {
		targetArch = &buildArch
	}

	localPackages := pkgjson.PackageRepo{}
	err = localPackages.ParsePackageJSON(*input)
	if err != nil {
		logger.Log.Panic(err)
	}

	err = populateGraph(depGraph, &localPackages, buildArch, *targetArch)
	if err != nil {
		logger.Log.Panic(err)
	}

	err = depGraph.MakeDAG()
	if err != nil {
		logger.Log.Panic(err)
	}

	// Add a default "ALL" goal to build everything local
	_, err = depGraph.AddGoalNode("ALL", nil, *targetArch, *strictGoals)
	if err != nil {
		logger.Log.Panic(err)
	}
	//TODO: Do we want to sub graph here to prune all non-target arch packages?

	err = pkggraph.WriteDOTGraphFile(depGraph, *output)
	if err != nil {
		logger.Log.Panic(err)
	}

	logger.Log.Info("Finished generating graph.")
}

// addUnresolvedPackage adds an unresolved node to the graph representing the
// packged described in the PackgetVer structure. Returns an error if the node
// could not be created.
func addUnresolvedPackage(g *pkggraph.PkgGraph, pkgVer *pkgjson.PackageVer, arch string) (newRunNode *pkggraph.PkgNode, err error) {
	logger.Log.Debugf("Adding unresolved %s", pkgVer)
	if *strictUnresolved {
		err = fmt.Errorf("strict-unresolved does not allow unresolved packages, attempting to add %s", pkgVer)
		return
	}

	nodes, err := g.FindBestPkgNode(pkgVer, arch)
	if err != nil {
		return
	}
	if nodes != nil {
		err = fmt.Errorf(`attempted to mark a local package "%+v" as unresolved`, pkgVer)
		return
	}

	// Create a new node
	newRunNode, err = g.AddPkgNode(pkgVer, pkggraph.StateUnresolved, pkggraph.TypeRemote, "<NO_SRPM_PATH>", "<NO_RPM_PATH>", "<NO_SPEC_PATH>", "<NO_SOURCE_PATH>", arch, "<NO_REPO>")
	if err != nil {
		return
	}

	logger.Log.Infof("Adding unresolved node %s\n", newRunNode.FriendlyName())

	return
}

// addNodesForPackage creates a "Run" and "Build" node for the package described
// in the PackageVer structure. Returns pointers to the build and run Nodes
// created, or an error if one of the nodes could not be created.
func addNodesForPackage(g *pkggraph.PkgGraph, pkgVer *pkgjson.PackageVer, pkg *pkgjson.Package) (newRunNode *pkggraph.PkgNode, newBuildNode *pkggraph.PkgNode, err error) {
	nodes, err := g.FindExactPkgNodeFromPkg(pkgVer, pkg.Architecture)
	if err != nil {
		return
	}
	if nodes != nil {
		logger.Log.Warnf(`Duplicate package name for package %+v read from SRPM "%s" (Previous: %+v)`, pkgVer, pkg.SrpmPath, nodes.RunNode)
		err = nil
		if nodes.RunNode != nil {
			newRunNode = nodes.RunNode
		}
		if nodes.BuildNode != nil {
			newBuildNode = nodes.BuildNode
		}
	}

	if newRunNode == nil {
		// Add "Run" node
		newRunNode, err = g.AddPkgNode(pkgVer, pkggraph.StateMeta, pkggraph.TypeRun, pkg.SrpmPath, pkg.RpmPath, pkg.SpecPath, pkg.SourceDir, pkg.Architecture, "<LOCAL>")
		logger.Log.Debugf("Adding run node %s with id %d\n", newRunNode.FriendlyName(), newRunNode.ID())
		if err != nil {
			return
		}
	}

	if newBuildNode == nil {
		// Add "Build" node
		newBuildNode, err = g.AddPkgNode(pkgVer, pkggraph.StateBuild, pkggraph.TypeBuild, pkg.SrpmPath, pkg.RpmPath, pkg.SpecPath, pkg.SourceDir, pkg.Architecture, "<LOCAL>")
		logger.Log.Debugf("Adding build node %s with id %d\n", newBuildNode.FriendlyName(), newBuildNode.ID())
		if err != nil {
			return
		}
	}

	// A "run" node has an implicit dependency on its coresponding "build" node, encode that here.
	// SetEdge panics on error, and does not support looping edges.
	newEdge := g.NewEdge(newRunNode, newBuildNode)
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Panicf("Adding edge failed for %+v", pkgVer)
		}
	}()
	g.SetEdge(newEdge)

	return
}

// addSingleDependency will add an edge between packageNode and the "Run" node for the
// dependency described in the PackageVer structure. Returns an error if the
// addition failed.
func addSingleDependency(g *pkggraph.PkgGraph, packageNode *pkggraph.PkgNode, dependency *pkgjson.PackageVer, arch string) error {
	var dependentNode *pkggraph.PkgNode
	logger.Log.Tracef("Adding a dependency from %+v to %+v", packageNode.VersionedPkg, dependency)
	nodes, err := g.FindBestPkgNode(dependency, arch)
	if err != nil {
		logger.Log.Errorf("Unable to check lookup list for %+v (%s)", dependency, err)
		return err
	}

	if nodes == nil {
		dependentNode, err = addUnresolvedPackage(g, dependency, arch)
		if err != nil {
			logger.Log.Errorf(`Could not add a package "%s"`, dependency.Name)
			return err
		}
	} else {
		// All dependencies are assumed to be "Run" dependencies
		dependentNode = nodes.RunNode
	}

	// SetEdge panics on error, and does not support looping edges.
	newEdge := g.NewEdge(packageNode, dependentNode)
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Errorf("Failed to add edge failed between %+v and %+v", packageNode, dependency)
		}
	}()
	if newEdge.To() == newEdge.From() {
		logger.Log.Debugf("Package %+v requires itself!", packageNode)
		return nil
	}

	// Avoid creating runtime dependencies from an RPM to a different provide from the same RPM as the dependency will always be met on RPM installation.
	// Creating these edges may cause non-problematic cycles that can significantly increase memory usage and runtime during cycle resolution.
	// If there are enough of these cycles it can exhaust the system's memory when resolving them.
	// - Only check run nodes. If a build node has a reflexive cycle then it cannot be built without a bootstrap version.
	if packageNode.Type == pkggraph.TypeRun &&
		dependentNode.Type == pkggraph.TypeRun &&
		packageNode.RpmPath == dependentNode.RpmPath {

		logger.Log.Debugf("%+v requires %+v which is provided by the same RPM.", packageNode, dependentNode)
		return nil
	}

	g.SetEdge(newEdge)

	return err
}

// addLocalPackage adds the package provided by the Package structure, and
// updates the SRPM path name
func addLocalPackage(g *pkggraph.PkgGraph, pkg *pkgjson.Package) error {
	_, _, err := addNodesForPackage(g, pkg.Provides, pkg)
	return err
}

// addDependencies adds edges for both build and runtime requirements for the
// package described in the Package structure. Returns an error if the edges
// could not be created.
func addPkgDependencies(g *pkggraph.PkgGraph, pkg *pkgjson.Package, buildArch, targetArch string) (dependenciesAdded int, err error) {
	provide := pkg.Provides
	runDependencies := pkg.Requires
	buildDependencies := pkg.BuildRequires

	// Find the current node in the lookup list.
	logger.Log.Debugf("Adding dependencies for package %s", pkg.SrpmPath)
	nodes, err := g.FindExactPkgNodeFromPkg(provide, pkg.Architecture)
	if err != nil {
		return
	}
	if nodes == nil {
		return dependenciesAdded, fmt.Errorf("can't add dependencies to a missing package %+v", pkg)
	}
	runNode := nodes.RunNode
	buildNode := nodes.BuildNode

	// For each run time and build time dependency, add the edges
	logger.Log.Tracef("Adding run dependencies")
	for _, dependency := range runDependencies {
		err = addSingleDependency(g, runNode, dependency, buildArch)
		if err != nil {
			logger.Log.Errorf("Unable to add run-time dependencies for %+v", pkg)
			return
		}
		dependenciesAdded++
		if buildArch != targetArch && pkg.Architecture == "noarch" {
			// We won't know where a noarch package will be installed, it may have
			// arch specific runtime requires so we need to build both build and target arches.
			err = addSingleDependency(g, runNode, dependency, targetArch)
			if err != nil {
				logger.Log.Errorf("Unable to add cross-arch run-time dependencies for %+v", pkg)
				return
			}
			dependenciesAdded++
		}
	}

	logger.Log.Tracef("Adding build dependencies")
	for _, dependency := range buildDependencies {
		// Most packages will both build and run on the buildArch architecture
		err = addSingleDependency(g, buildNode, dependency, buildArch)
		if err != nil {
			logger.Log.Errorf("Unable to add build-time dependencies for %+v", pkg)
			return
		}
		dependenciesAdded++
		// For either cross arch, or noarch packages we may also need the target arch packages to build
		// if we are cross compiling.
		if buildArch != targetArch && pkg.Architecture != buildArch {
			err = addSingleDependency(g, buildNode, dependency, targetArch)
			if err != nil {
				logger.Log.Errorf("Unable to add cross-arch build-time dependencies for %+v", pkg)
				return
			}
			dependenciesAdded++
		}
	}

	return
}

// populateGraph adds all the data contained in the PackageRepo structure into
// the graph.
func populateGraph(g *pkggraph.PkgGraph, repo *pkgjson.PackageRepo, buildArch, targetArch string) (err error) {
	packages := repo.Repo

	// Scan and add each package we know about
	logger.Log.Infof("Adding all packages from %s", *input)
	// NOTE: range iterates by value, not reference. Manually access slice
	for idx := range packages {
		pkg := packages[idx]
		err = addLocalPackage(g, pkg)
		if err != nil {
			logger.Log.Errorf("Failed to add local package %+v", pkg)
			return err
		}
	}
	logger.Log.Infof("\tAdded %d packages", len(packages))

	// Rescan and add all the dependencies
	logger.Log.Infof("Adding all dependencies from %s", *input)
	dependenciesAdded := 0
	for idx := range packages {
		pkg := packages[idx]
		num, err := addPkgDependencies(g, pkg, buildArch, targetArch)
		if err != nil {
			logger.Log.Errorf("Failed to add dependency %+v", pkg)
			return err
		}
		dependenciesAdded += num
	}
	logger.Log.Infof("\tAdded %d dependencies", dependenciesAdded)

	return err
}
