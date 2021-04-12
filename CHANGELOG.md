# v0.2.0

* Introduce supervision restart strategy `OneForAll` (#65)

* `SupervisorRestartError` explain message got enhanced with first detected error
  on a window of time (#65)

* Introduce `NewDynSubtree` and `NewDynSubtreeWithNotifyStart` to create managed
  subtrees with dynamic worker spawning capabilities. (#66)

* Bump go version to 1.16 (#67)

# v0.1.0 (breaking changes)

* Deprecate `WithTolerance` worker option with `WithRestartTolerance` supervisor
  option #breaking-change (#55)

* Move all files in the `cap` folder to `internal/s` and do an explicit export
  list of symbols (#56)

* Add a new `EventNotifier` called `ReliableNotifier`, which guarantees a safe,
  failure tolerant, latency tolerant event notifier dispatching mechanism. #new (#58)

* Expose the `NodeSepToken` variable to join symbols from a tree hierarchy.
  #new (#58)

* Add new `EventCriteria` combinator which allows us to easily modify
  `EventNotifier` values to accept a subset of events #new (#58)

* Add `ExplainError` function to get a human-friendly error explanation for
  Capataz errors #new (#62)

# v0.0.0

* See changes on [Every
  PR](https://github.com/capatazlib/go-capataz/pulls?q=is%3Apr+is%3Aclosed+label%3Apre-changelog)
  that was created before a CHANGELOG file was added
