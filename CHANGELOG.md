# pre-release v0.1.0 (breaking changes)

* Deprecate `WithTolerance` worker option with `WithRestartTolerance` supervisor
  option (#55)

* Move all files in the `cap` folder to `internal/s` and do an explicit export
  list of symbols (#56)

* Attach multiple event notifiers by passing multiple cap.WithNotifier options
  to a supervisor (#57)

* Support attaching an event notifier to a subtree to receive events only for
  that part of the supervision tree (#57)

# v0.0.0

* See changes on [Every PR](https://github.com/capatazlib/go-capataz/pulls?q=is%3Apr+is%3Aclosed+label%3Apre-changelog) that was created before a CHANGELOG file was added
