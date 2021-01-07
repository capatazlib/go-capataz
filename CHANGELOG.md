## v0.0.1

* Introducing `NewDynSubtree` to allow workers that can spawn workers dynamically

* Ensure test `EventManager` event collection is concurrent-safe

* Ensure `Terminate` and `Wait` calls on `DynSupervisors` are idempotent

* Fix `ObserveSupervisor` functions to not leak event collector goroutines

* Rename of internal variables for enhanced readbility

## v0.0.0

* No consistent tracking of changes before release, please check closed PR's
  up till #53
