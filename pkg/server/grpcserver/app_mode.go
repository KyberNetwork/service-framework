package grpcserver

type AppMode string

// A list of app modes.
const (
	Development AppMode = "develop"
	Production  AppMode = "production"
)
