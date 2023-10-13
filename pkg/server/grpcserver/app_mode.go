package grpcserver

//go:generate go run github.com/dmarkham/enumer -type=AppMode -json
type AppMode int

// A list of app modes.
const (
	Development AppMode = iota
	Production
)

func GetAppMode(appMode string) AppMode {
	mode, err := AppModeString(appMode)
	if err != nil {
		return Development
	}
	return mode
}
