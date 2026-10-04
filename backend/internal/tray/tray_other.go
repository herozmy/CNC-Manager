//go:build !windows

package tray

func startPlatform(actions *guardedActions) (func(), error) {
	return func() {}, nil
}
