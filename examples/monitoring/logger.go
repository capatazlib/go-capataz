package main

import (
	"os"

	"github.com/sirupsen/logrus"

	"github.com/capatazlib/go-capataz/s"
)

func newLogEventNotifier() (*logrus.Entry, s.EventNotifier) {
	log := logrus.New()
	log.Out = os.Stdout
	log.Level = logrus.DebugLevel
	log.SetFormatter(&logrus.JSONFormatter{})

	ll := log.WithFields(logrus.Fields{})

	return ll, func(ev s.Event) {
		if ev.Err() != nil {
			ll = log.WithError(ev.Err())
		}
		ll.WithFields(logrus.Fields{
			"process_runtime_name": ev.ProcessRuntimeName(),
			"created_at":           ev.Created(),
		}).Debug(ev.Tag().String())
	}
}
