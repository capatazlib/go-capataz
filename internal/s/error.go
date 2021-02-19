package s

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/capatazlib/go-capataz/internal/c"
)

// terminateNodeError is the error reported back to a Supervisor when the
// termination of a node fails
type terminateNodeError = error

// startNodeError is the error reported back to a Supervisor when the start of a
// node fails
type startNodeError = error

// ErrKVs is an utility interface used to get key-values out of Capataz errors
type ErrKVs interface {
	KVs() map[string]interface{}
}

// errExplain is an utility interface used to get a human-friendly message from
// a Capataz error
type errExplain interface {
	explainLines() []string
}

// SupervisorTerminationError wraps errors returned by a child node that failed
// to terminate (io errors, timeouts, etc.), enhancing it with supervisor
// information. Note, the only way to have a valid SupervisorTerminationError is
// for one of the child nodes to fail or the supervisor cleanup operation fails.
type SupervisorTerminationError struct {
	supRuntimeName string
	nodeErrMap     map[string]error
	rscCleanupErr  error
}

// Error returns an error message
func (err *SupervisorTerminationError) Error() string {
	return "supervisor terminated with failures"
}

// KVs returns a metadata map for structured logging
func (err *SupervisorTerminationError) KVs() map[string]interface{} {
	nodeNames := make([]string, 0, len(err.nodeErrMap))
	for nodeName := range err.nodeErrMap {
		nodeNames = append(nodeNames, nodeName)
	}
	sort.Strings(nodeNames)

	acc := make(map[string]interface{})
	acc["supervisor.name"] = err.supRuntimeName

	for i, nodeName := range nodeNames {
		nodeErr := err.nodeErrMap[nodeName]
		var subTreeError ErrKVs
		if errors.As(nodeErr, &subTreeError) {
			for k0, v := range subTreeError.KVs() {
				k := strings.TrimPrefix(k0, "supervisor.")
				acc[fmt.Sprintf("supervisor.subtree.%d.%s", i, k)] = v
			}
		} else {
			acc[fmt.Sprintf("supervisor.termination.node.%d.name", i)] = nodeName
			acc[fmt.Sprintf("supervisor.termination.node.%d.error", i)] = nodeErr
		}

	}

	if err.rscCleanupErr != nil {
		acc["supervisor.termination.cleanup.error"] = err.rscCleanupErr
	}

	return acc
}

// explainLines returns a human-friendly message of the error represented as a slice
// of lines
func (err *SupervisorTerminationError) explainLines() []string {
	// error reporting from children
	var nodeErrLines []string

	// deal with a single child error
	if len(err.nodeErrMap) == 1 {
		for childName, childErr := range err.nodeErrMap {
			// deal with sub-tree termination errors
			if errExplain, ok := childErr.(errExplain); ok {
				nodeErrLines = append(nodeErrLines, errExplain.explainLines()...)
			} else {
				// deal with worker termination error
				nodeErrLines = append(
					nodeErrLines,
					fmt.Sprintf("worker node '%s%s%s' failed to terminate",
						err.supRuntimeName,
						NodeSepToken,
						childName,
					),
				)
				nodeErrLines = append(
					nodeErrLines,
					indentExplain(1, errToExplain(childErr))...,
				)
			}
		}

		if err.rscCleanupErr != nil {
			errLines := indentExplain(1, errToExplain(err.rscCleanupErr))
			nodeErrLines = append(nodeErrLines,
				"also, cleanup failed of the supervisor failed:",
			)
			nodeErrLines = append(nodeErrLines,
				errLines...,
			)
		}

		return nodeErrLines
	}

	if len(err.nodeErrMap) == 0 && err.rscCleanupErr != nil {
		errLines := indentExplain(1, errToExplain(err.rscCleanupErr))
		nodeErrLines = append(nodeErrLines,
			fmt.Sprintf("supervisor '%s' cleanup failed on termination", err.supRuntimeName),
		)
		nodeErrLines = append(nodeErrLines,
			errLines...,
		)

		return nodeErrLines
	}

	if len(err.nodeErrMap) > 0 {
		// report for sub-tree errors
		var subtreeErrLines = make([]string, 0)
		// report for direct children error
		var workerErrLines = make([]string, 0)

		for childName, childErr := range err.nodeErrMap {
			// deal with sub-tree termination errors
			if errExplain, ok := childErr.(errExplain); ok {
				subtreeErrLines = append(subtreeErrLines, errExplain.explainLines()...)
			} else {
				// deal with worker termination error
				workerErrLines = append(
					workerErrLines,
					fmt.Sprintf("the worker node '%s%s%s' failed to terminate:",
						err.supRuntimeName,
						NodeSepToken,
						childName,
					),
				)
				workerErrLines = append(
					workerErrLines,
					indentExplain(1, errToExplain(childErr))...,
				)
			}
		}

		nodeErrLines = append(nodeErrLines,
			fmt.Sprintf(
				"children cleanup failed: %d node(s) failed to terminate",
				len(err.nodeErrMap),
			),
		)

		if len(subtreeErrLines) > 0 {
			nodeErrLines = append(
				nodeErrLines, indentExplain(1, subtreeErrLines)...,
			)
		}

		if len(workerErrLines) > 0 {
			nodeErrLines = append(
				nodeErrLines, indentExplain(1, workerErrLines)...,
			)
		}
	}

	// error reporting from cleanup
	var cleanupErrLines []string
	if err.rscCleanupErr != nil {
		errLines := indentExplain(1, errToExplain(err.rscCleanupErr))
		cleanupErrLines = append(cleanupErrLines,
			"cleanup failed:",
		)
		cleanupErrLines = append(cleanupErrLines,
			errLines...,
		)
	}

	var outputLines []string
	outputLines = append(
		outputLines,
		"supervisor failed to terminate",
	)

	if len(nodeErrLines) > 0 {
		outputLines = append(
			outputLines,
			indentExplain(1, nodeErrLines)...,
		)
	}

	if len(cleanupErrLines) > 0 {
		outputLines = append(
			outputLines,
			indentExplain(1, cleanupErrLines)...,
		)
	}

	return outputLines
}

// SupervisorBuildError wraps errors returned from a client provided function
// that builds the supervisor nodes, enhancing it with supervisor information
type SupervisorBuildError struct {
	supRuntimeName string
	buildNodesErr  error
}

func (err *SupervisorBuildError) Error() string {
	return "supervisor build nodes function failed"
}

// KVs returns a metadata map for structured logging
func (err *SupervisorBuildError) KVs() map[string]interface{} {
	acc := make(map[string]interface{})
	acc["supervisor.name"] = err.supRuntimeName
	acc["supervisor.build.error"] = err.buildNodesErr
	return acc
}

// explainLines returns a human-friendly message of the error represented as a slice
// of lines
func (err *SupervisorBuildError) explainLines() []string {
	var outputLines []string

	outputLines = append(
		outputLines,
		fmt.Sprintf("supervisor '%s' build nodes function failed", err.supRuntimeName),
	)

	outputLines = append(
		outputLines,
		indentExplain(1, errToExplain(err.buildNodesErr))...,
	)

	return outputLines
}

// SupervisorStartError wraps an error reported on the initialization of a child
// node, enhancing it with supervisor information and possible termination errors
// on other siblings
type SupervisorStartError struct {
	supRuntimeName string
	nodeName       string
	nodeErr        error
	// terminationErr is non-nil when the abort process triggered by a supervisor
	// start error produced new errors. A SupervisorTerminationError value will
	// only exists when at least one supervisor node failed to terminate.
	terminationErr *SupervisorTerminationError
}

// Error returns an error message
func (err *SupervisorStartError) Error() string {
	return "supervisor node failed to start"
}

// KVs returns a metadata map for structured logging
func (err *SupervisorStartError) KVs() map[string]interface{} {
	acc := make(map[string]interface{})
	acc["supervisor.name"] = err.supRuntimeName

	if err.nodeErr != nil {
		var subTreeError ErrKVs
		if errors.As(err.nodeErr, &subTreeError) {
			for k0, v := range subTreeError.KVs() {
				k := strings.TrimPrefix(k0, "supervisor.")
				acc[fmt.Sprintf("supervisor.subtree.%s", k)] = v
			}
		} else {
			acc["supervisor.start.node.name"] = err.nodeName
			acc["supervisor.start.node.error"] = err.nodeErr
		}
	}

	if err.terminationErr != nil {
		for k, v := range err.terminationErr.KVs() {
			acc[k] = v
		}
	}

	return acc
}

func (err *SupervisorStartError) explainLines() []string {
	var workerErrLines []string

	if errExplain, ok := err.nodeErr.(errExplain); ok {
		workerErrLines = append(
			workerErrLines,
			errExplain.explainLines()...,
		)
	} else {
		workerErrLines = append(
			workerErrLines,
			fmt.Sprintf("supervisor failed to start\n"),
		)
		workerErrLines = append(
			workerErrLines,
			fmt.Sprintf("\tworker node '%s%s%s' failed to start",
				err.supRuntimeName, NodeSepToken, err.nodeName),
		)
		workerErrLines = append(
			workerErrLines,
			indentExplain(2, errToExplain(err.nodeErr))...,
		)
	}

	var terminationErrLines []string
	if err.terminationErr != nil {
		terminationErrLines = append(
			terminationErrLines,
			"\n\talso, some previously started siblings failed to terminate",
		)

		terminationErrLines = append(
			terminationErrLines,
			indentExplain(2, err.terminationErr.explainLines())...,
		)
	}

	var outputLines []string

	outputLines = append(
		outputLines,
		workerErrLines...,
	)

	if len(terminationErrLines) > 0 {
		outputLines = append(
			outputLines,
			terminationErrLines...,
		)
	}

	return outputLines
}

// SupervisorRestartError wraps an error tolerance surpassed error from a child
// node, enhancing it with supervisor information and possible termination errors
// on other siblings
type SupervisorRestartError struct {
	supRuntimeName string
	nodeErr        *RestartToleranceReached
	terminationErr *SupervisorTerminationError
}

// Error returns an error message
func (err *SupervisorRestartError) Error() string {
	return "supervisor crashed due to restart tolerance surpassed"
}

// KVs returns a metadata map for structured logging
func (err *SupervisorRestartError) KVs() map[string]interface{} {
	acc := make(map[string]interface{})
	acc["supervisor.name"] = err.supRuntimeName

	if err.nodeErr != nil {
		for k, v := range err.nodeErr.KVs() {
			acc[fmt.Sprintf("supervisor.restart.%s", k)] = v
		}
	}

	if err.terminationErr != nil {
		for k, v := range err.terminationErr.KVs() {
			acc[k] = v
		}
	}

	return acc
}

// explainLines returns a human-friendly message of the error represented as a slice
// of lines
func (err *SupervisorRestartError) explainLines() []string {
	var outputLines []string

	outputLines = append(
		outputLines,
		fmt.Sprintf(
			"supervisor '%s' crashed due to restart tolerance surpassed.",
			err.supRuntimeName,
		),
	)

	outputLines = append(
		outputLines,
		indentExplain(1, err.nodeErr.explainLines())...,
	)

	if err.terminationErr != nil {
		outputLines = append(
			outputLines,
			"also, some siblings failed to terminate while restarting",
		)
		outputLines = append(
			outputLines,
			indentExplain(1, err.terminationErr.explainLines())...,
		)
	}

	return outputLines
}

// RestartToleranceReached is an error that gets reported when a supervisor has
// restarted a child so many times over a period of time that it does not make
// sense to keep restarting.
type RestartToleranceReached struct {
	failedChildName        string
	failedChildErrCount    uint32
	failedChildErrDuration time.Duration
	err                    error
}

// NewRestartToleranceReached creates an ErrorToleranceReached record
func NewRestartToleranceReached(
	tolerance restartTolerance,
	err error,
	ch c.Child,
) *RestartToleranceReached {
	return &RestartToleranceReached{
		failedChildName:        ch.GetRuntimeName(),
		failedChildErrCount:    tolerance.MaxRestartCount,
		failedChildErrDuration: tolerance.RestartWindow,
		err:                    err,
	}
}

// KVs returns a data bag map that may be used in structured logging
func (err *RestartToleranceReached) KVs() map[string]interface{} {
	kvs := make(map[string]interface{})
	kvs["node.name"] = err.failedChildName
	if err.err != nil {
		kvs["node.error.msg"] = err.err.Error()
		kvs["node.error.count"] = err.failedChildErrCount
		kvs["node.error.duration"] = err.failedChildErrDuration
	}
	return kvs
}

func (err *RestartToleranceReached) Error() string {
	return "node failures surpassed restart tolerance"
}

// Unwrap returns the last error that caused the creation of an
// ErrorToleranceReached error
func (err *RestartToleranceReached) Unwrap() error {
	return err.err
}

// explainLines returns a human-friendly message of the error represented as a slice
// of lines
func (err *RestartToleranceReached) explainLines() []string {
	var outputLines []string
	outputLines = append(
		outputLines,
		[]string{
			fmt.Sprintf(
				"worker node '%s' was restarted at least %d times in a %v window; "+
					"the last error reported was:",
				err.failedChildName,
				err.failedChildErrCount,
				err.failedChildErrDuration,
			),
		}...,
	)
	outputLines = append(
		outputLines,
		indentExplain(1, errToExplain(err.err))...,
	)
	return outputLines
}

////////////////////

// ExplainError is an utility function that explains capataz errors in a human-friendly
// way. Defaults to a call to error.Error() if the underlying error does not come from
// the capataz library
func ExplainError(err error) string {
	if errExp, ok := err.(errExplain); ok {
		return strings.Join(errExp.explainLines(), "\n")
	}
	return err.Error()
}

// errToExplain transforms an error message into a human-friendly readbable
// string
func errToExplain(err error) []string {
	errLines := strings.Split(err.Error(), "\n")
	for i, l := range errLines {
		errLines[i] = fmt.Sprintf("> %s", l)
	}
	return errLines
}

// indentExplain indents the given lines a number of specified times
func indentExplain(times int, ss []string) []string {
	indentPrefix := strings.Repeat("\t", times)
	for i, s := range ss {
		ss[i] = fmt.Sprintf("%s%s", indentPrefix, s)
	}
	return ss
}
