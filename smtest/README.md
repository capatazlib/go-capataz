# State Machine (sm) Testing

This is an utility library that help developers collect notification events from
an state machine in a concurrent-safe way, and help them assert these events
match a given predicate and are delivered in a predetermined order.

This utility is used extensively by the Capataz library, but it can also be used
by other concurrent algorithms that collect multiple events over callbacks.
